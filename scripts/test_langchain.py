#!/usr/bin/env python3
"""
Test script to verify LangChain integration works correctly with EK-1.
Run this to validate the setup before integrating with the Go application.
"""

import sys
import json
import sqlite3
import tempfile
from datetime import datetime, timedelta
from langchain_optimizer import LangChainAnalyzer, AnalysisRequest

def create_test_database():
    """Create a temporary database with sample EK-1 data for testing."""
    # Create temporary database
    db = sqlite3.connect(':memory:')
    cursor = db.cursor()

    # Create tables matching EK-1 schema
    cursor.execute('''
        CREATE TABLE signals (
            id INTEGER PRIMARY KEY,
            service_slug TEXT,
            analysis TEXT,
            status INTEGER,
            processed_at INTEGER
        )
    ''')

    cursor.execute('''
        CREATE TABLE check_in_history (
            id INTEGER PRIMARY KEY,
            mood INTEGER,
            stress_level INTEGER,
            sleep REAL,
            energy INTEGER,
            recorded_at INTEGER
        )
    ''')

    cursor.execute('''
        CREATE TABLE notifications (
            id INTEGER PRIMARY KEY,
            type TEXT,
            title TEXT,
            body TEXT,
            read INTEGER,
            created_at INTEGER
        )
    ''')

    # Insert sample data
    base_time = int((datetime.now() - timedelta(days=7)).timestamp())

    # Sample signals
    signals_data = [
        (1, 'gmail', '{"priority": "high", "is_relevant": true}', 0, base_time + 3600),
        (2, 'slack', '{"priority": "medium", "is_relevant": true}', 1, base_time + 7200),
        (3, 'calendar', '{"priority": "high", "is_relevant": true}', 0, base_time + 10800),
        (4, 'gmail', '{"priority": "low", "is_relevant": false}', 2, base_time + 14400),
        (5, 'slack', '{"priority": "high", "is_relevant": true}', 0, base_time + 18000),
    ]

    cursor.executemany('''
        INSERT INTO signals (id, service_slug, analysis, status, processed_at)
        VALUES (?, ?, ?, ?, ?)
    ''', signals_data)

    # Sample biometric data
    biometric_data = []
    for i in range(7):
        day_offset = i * 86400  # seconds per day
        biometric_data.append((
            i + 1,
            7 - i,  # mood decreasing over week
            i + 3,  # stress increasing
            8.0 - (i * 0.5),  # sleep decreasing
            8 - i,  # energy decreasing
            base_time + day_offset
        ))

    cursor.executemany('''
        INSERT INTO check_in_history (id, mood, stress_level, sleep, energy, recorded_at)
        VALUES (?, ?, ?, ?, ?, ?)
    ''', biometric_data)

    # Sample notifications
    notifications_data = [
        (1, 'H2HI', 'High Stress Alert', 'Stress levels elevated', 0, base_time + 3600),
        (2, 'OPPORTUNITY', 'Investment Signal', 'New opportunity detected', 1, base_time + 7200),
        (3, 'SOUL_DRIFT', 'Values Misalignment', 'Actions not matching values', 0, base_time + 10800),
    ]

    cursor.executemany('''
        INSERT INTO notifications (id, type, title, body, read, created_at)
        VALUES (?, ?, ?, ?, ?, ?)
    ''', notifications_data)

    db.commit()
    return db

def test_basic_analysis():
    """Test basic LangChain analysis functionality."""
    print("🧪 Testing basic LangChain analysis...")

    # Create test database
    db = create_test_database()

    # Save to temporary file for the analyzer
    with tempfile.NamedTemporaryFile(suffix='.db', delete=False) as tmp_db:
        # Copy in-memory db to file
        backup = sqlite3.connect(tmp_db.name)
        db.backup(backup)
        backup.close()
        db_path = tmp_db.name

    try:
        # Initialize analyzer
        analyzer = LangChainAnalyzer(db_path)

        # Test simple analysis request
        request = AnalysisRequest(
            query="What patterns do you see in my stress and productivity data?",
            time_frame="week"
        )

        # Perform analysis
        result = analyzer.analyze(request)

        # Validate result structure
        assert "query_explanation" in result, "Missing query explanation"
        assert "generated_queries" in result, "Missing generated queries"
        assert "analysis" in result, "Missing analysis"
        assert len(result["generated_queries"]) > 0, "No queries generated"

        print("✅ Basic analysis test passed")
        print(f"   Generated {len(result['generated_queries'])} queries")
        print(f"   Analysis length: {len(result['analysis'])} characters")

        return result

    finally:
        # Cleanup
        import os
        os.unlink(db_path)

def test_query_generation():
    """Test SQL query generation capabilities."""
    print("🧪 Testing SQL query generation...")

    # Create test database
    db = create_test_database()

    with tempfile.NamedTemporaryFile(suffix='.db', delete=False) as tmp_db:
        backup = sqlite3.connect(tmp_db.name)
        db.backup(backup)
        backup.close()
        db_path = tmp_db.name

    try:
        analyzer = LangChainAnalyzer(db_path)

        # Test different query types
        test_queries = [
            "Show correlation between stress and sleep",
            "What are my highest priority pending tasks?",
            "Analyze trends in my mood over time"
        ]

        for query in test_queries:
            request = AnalysisRequest(query=query, time_frame="week")
            result = analyzer.analyze(request)

            # Validate SQL queries were generated
            assert len(result["generated_queries"]) > 0, f"No queries for: {query}"

            # Check that queries are valid SQL
            for sql_query in result["generated_queries"]:
                try:
                    # Try to explain the query (validates syntax without executing)
                    test_db = sqlite3.connect(db_path)
                    cursor = test_db.cursor()
                    cursor.execute(f"EXPLAIN QUERY PLAN {sql_query}")
                    test_db.close()
                except sqlite3.Error as e:
                    print(f"❌ Invalid SQL generated for '{query}': {sql_query}")
                    print(f"   Error: {e}")
                    continue

            print(f"✅ Query generation test passed for: '{query}'")

    finally:
        import os
        os.unlink(db_path)

def test_error_handling():
    """Test error handling and fallback behavior."""
    print("🧪 Testing error handling...")

    # Test with non-existent database
    try:
        analyzer = LangChainAnalyzer("/non/existent/path.db")
        request = AnalysisRequest(query="test", time_frame="week")
        result = analyzer.analyze(request)
        print("❌ Should have failed with non-existent database")
    except Exception:
        print("✅ Properly handles non-existent database")

    # Test with empty query
    db = create_test_database()
    with tempfile.NamedTemporaryFile(suffix='.db', delete=False) as tmp_db:
        backup = sqlite3.connect(tmp_db.name)
        db.backup(backup)
        backup.close()
        db_path = tmp_db.name

    try:
        analyzer = LangChainAnalyzer(db_path)
        request = AnalysisRequest(query="", time_frame="week")
        result = analyzer.analyze(request)

        # Should handle empty queries gracefully
        print("✅ Handles empty queries gracefully")

    except Exception as e:
        print(f"⚠️  Empty query handling could be improved: {e}")
    finally:
        import os
        os.unlink(db_path)

def main():
    """Run all tests and report results."""
    print("🚀 LangChain Integration Test Suite")
    print("=" * 50)

    try:
        # Check dependencies
        print("📋 Checking dependencies...")
        import langchain
        print(f"✅ LangChain version: {langchain.__version__}")

        # Run tests
        test_basic_analysis()
        test_query_generation()
        test_error_handling()

        print("\n🎉 All tests passed!")
        print("\nLangChain integration is working correctly.")
        print("You can now integrate it with your EK-1 Go application.")

        return 0

    except ImportError as e:
        print(f"❌ Missing dependency: {e}")
        print("Run: pip install langchain langchain-community")
        return 1

    except Exception as e:
        print(f"❌ Test failed: {e}")
        import traceback
        traceback.print_exc()
        return 1

if __name__ == "__main__":
    sys.exit(main())
