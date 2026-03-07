#!/usr/bin/env python3
"""
LangChain-powered SQL query optimizer for EK-1 data analysis.
Provides sophisticated query generation and multi-step reasoning.
"""

import json
import sqlite3
import sys
from datetime import datetime, timedelta
from typing import Dict, List, Any, Optional
from dataclasses import dataclass

from langchain.llms import Ollama
from langchain.prompts import PromptTemplate
from langchain.chains import LLMChain
from langchain.tools import Tool
from langchain.agents import initialize_agent, AgentType
from langchain.schema import BaseOutputParser
from langchain.memory import ConversationBufferMemory


@dataclass
class AnalysisRequest:
    query: str
    time_frame: str = "week"
    focus: List[str] = None
    user_context: Dict[str, Any] = None


class SQLQueryParser(BaseOutputParser):
    """Parse LLM output to extract SQL queries and explanations."""

    def parse(self, text: str) -> Dict[str, str]:
        lines = text.strip().split('\n')
        sql_queries = []
        explanation = ""

        in_sql = False
        current_query = []

        for line in lines:
            line = line.strip()
            if line.lower().startswith('```sql'):
                in_sql = True
                continue
            elif line.lower().startswith('```'):
                in_sql = False
                if current_query:
                    sql_queries.append('\n'.join(current_query))
                    current_query = []
                continue
            elif in_sql:
                current_query.append(line)
            else:
                explanation += line + "\n"

        return {
            "queries": sql_queries,
            "explanation": explanation.strip()
        }


class EK1DatabaseTool:
    """Enhanced database interface with schema awareness."""

    def __init__(self, db_path: str):
        self.db_path = db_path
        self.schema = self._get_schema()

    def _get_schema(self) -> Dict[str, List[str]]:
        """Extract database schema for LLM context."""
        conn = sqlite3.connect(self.db_path)
        cursor = conn.cursor()

        schema = {}

        # Get all tables
        cursor.execute("SELECT name FROM sqlite_master WHERE type='table';")
        tables = cursor.fetchall()

        for (table_name,) in tables:
            cursor.execute(f"PRAGMA table_info({table_name})")
            columns = cursor.fetchall()
            schema[table_name] = [
                f"{col[1]} {col[2]}" + (" PRIMARY KEY" if col[5] else "")
                for col in columns
            ]

        conn.close()
        return schema

    def execute_query(self, query: str) -> List[Dict[str, Any]]:
        """Execute SQL query and return results as dictionaries."""
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row  # Enable column access by name
        cursor = conn.cursor()

        try:
            cursor.execute(query)
            results = [dict(row) for row in cursor.fetchall()]
            conn.close()
            return results
        except Exception as e:
            conn.close()
            raise Exception(f"SQL Error: {e}")

    def get_schema_context(self) -> str:
        """Generate schema description for LLM prompts."""
        context = "# Database Schema\n\n"
        for table, columns in self.schema.items():
            context += f"## {table}\n"
            context += "```sql\n"
            context += f"CREATE TABLE {table} (\n"
            context += ",\n".join(f"  {col}" for col in columns)
            context += "\n);\n```\n\n"
        return context


class LangChainAnalyzer:
    """Main LangChain-powered analyzer."""

    def __init__(self, db_path: str, ollama_host: str = "http://localhost:11434", model: str = "llama3.2"):
        self.db_tool = EK1DatabaseTool(db_path)
        self.llm = Ollama(base_url=ollama_host, model=model, temperature=0)
        self.memory = ConversationBufferMemory(memory_key="chat_history", return_messages=True)
        self._setup_chains()
        self._setup_agent()

    def _setup_chains(self):
        """Initialize LangChain chains for different analysis tasks."""

        # Query Generation Chain
        query_template = PromptTemplate(
            input_variables=["schema", "user_request", "time_frame"],
            template="""You are an expert SQL analyst for a personal AI agent system.

{schema}

User Request: {user_request}
Time Frame: {time_frame}

Generate optimized SQL queries to answer the user's request. Consider:
1. Focus on actionable insights and patterns
2. Use appropriate time windows (week/month/quarter)
3. Join tables when necessary for comprehensive analysis
4. Include aggregations and trend analysis
5. Order results by relevance/priority

Provide:
1. 1-3 optimized SQL queries
2. Brief explanation of what each query reveals

Format your response as:
```sql
-- Query 1: Description
SELECT ... FROM ... WHERE ... ORDER BY ...
```

```sql
-- Query 2: Description
SELECT ... FROM ... WHERE ... ORDER BY ...
```

Explanation: What insights these queries provide and why they're relevant.
"""
        )

        self.query_chain = LLMChain(llm=self.llm, prompt=query_template)

        # Analysis Chain
        analysis_template = PromptTemplate(
            input_variables=["query_results", "user_request"],
            template="""You are analyzing personal data for actionable insights.

User Request: {user_request}

Query Results:
{query_results}

Provide a comprehensive analysis including:
1. **TOP 3 PRIORITY AREAS** - What needs immediate attention
2. **PATTERNS IDENTIFIED** - Trends and correlations in the data
3. **SPECIFIC ACTIONS** - Time-bound, actionable recommendations
4. **RISK FACTORS** - Any concerning patterns (stress, overdue items, etc.)

Be specific, actionable, and focus on what the user can do today.
"""
        )

        self.analysis_chain = LLMChain(llm=self.llm, prompt=analysis_template)

    def _setup_agent(self):
        """Setup LangChain agent with database tools."""

        def query_database(query: str) -> str:
            """Execute SQL query and return formatted results."""
            try:
                results = self.db_tool.execute_query(query)
                if not results:
                    return "No results found."

                # Format results for LLM consumption
                if len(results) <= 10:
                    return json.dumps(results, indent=2, default=str)
                else:
                    # Summarize large result sets
                    summary = {
                        "total_rows": len(results),
                        "sample_rows": results[:5],
                        "statistics": self._generate_stats(results)
                    }
                    return json.dumps(summary, indent=2, default=str)
            except Exception as e:
                return f"Error executing query: {e}"

        def get_schema() -> str:
            """Get database schema information."""
            return self.db_tool.get_schema_context()

        tools = [
            Tool(
                name="query_database",
                description="Execute SQL queries on the EK-1 database. Input should be a valid SQL query.",
                func=query_database
            ),
            Tool(
                name="get_schema",
                description="Get the database schema to understand available tables and columns.",
                func=get_schema
            )
        ]

        self.agent = initialize_agent(
            tools=tools,
            llm=self.llm,
            agent=AgentType.CONVERSATIONAL_REACT_DESCRIPTION,
            memory=self.memory,
            verbose=True
        )

    def _generate_stats(self, results: List[Dict]) -> Dict[str, Any]:
        """Generate statistics for large result sets."""
        if not results:
            return {}

        stats = {"total_count": len(results)}

        # Analyze numeric columns
        for key in results[0].keys():
            values = [r[key] for r in results if r[key] is not None]
            if values and isinstance(values[0], (int, float)):
                stats[f"{key}_avg"] = sum(values) / len(values)
                stats[f"{key}_min"] = min(values)
                stats[f"{key}_max"] = max(values)

        return stats

    def analyze(self, request: AnalysisRequest) -> Dict[str, Any]:
        """Perform comprehensive data analysis using LangChain."""

        # Step 1: Generate optimized queries
        schema_context = self.db_tool.get_schema_context()

        query_response = self.query_chain.run(
            schema=schema_context,
            user_request=request.query,
            time_frame=request.time_frame
        )

        parser = SQLQueryParser()
        parsed = parser.parse(query_response)

        # Step 2: Execute queries and collect results
        all_results = {}
        for i, query in enumerate(parsed["queries"]):
            try:
                results = self.db_tool.execute_query(query)
                all_results[f"query_{i+1}"] = {
                    "sql": query,
                    "results": results,
                    "count": len(results)
                }
            except Exception as e:
                all_results[f"query_{i+1}"] = {
                    "sql": query,
                    "error": str(e)
                }

        # Step 3: Generate final analysis
        results_summary = json.dumps(all_results, indent=2, default=str)

        analysis_response = self.analysis_chain.run(
            query_results=results_summary,
            user_request=request.query
        )

        return {
            "query_explanation": parsed["explanation"],
            "generated_queries": parsed["queries"],
            "query_results": all_results,
            "analysis": analysis_response,
            "timestamp": datetime.now().isoformat()
        }

    def conversational_analysis(self, user_input: str) -> str:
        """Use agent for conversational, multi-step analysis."""
        return self.agent.run(user_input)


def main():
    """CLI interface for the LangChain analyzer."""
    if len(sys.argv) < 2:
        print("Usage: python langchain_optimizer.py '<user_query>' [time_frame] [db_path]")
        print("Example: python langchain_optimizer.py 'What should I prioritize today?' week")
        sys.exit(1)

    user_query = sys.argv[1]
    time_frame = sys.argv[2] if len(sys.argv) > 2 else "week"
    db_path = sys.argv[3] if len(sys.argv) > 3 else "./ek1.db"

    # Initialize analyzer
    analyzer = LangChainAnalyzer(db_path)

    # Create analysis request
    request = AnalysisRequest(
        query=user_query,
        time_frame=time_frame
    )

    try:
        # Perform analysis
        result = analyzer.analyze(request)

        # Output results as JSON for Go consumption
        print(json.dumps(result, indent=2, default=str))

    except Exception as e:
        error_result = {
            "error": str(e),
            "timestamp": datetime.now().isoformat()
        }
        print(json.dumps(error_result, indent=2))
        sys.exit(1)


if __name__ == "__main__":
    main()
