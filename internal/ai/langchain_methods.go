package ai

import "context"

// IsLangChainEnabled returns whether LangChain is available
func (e *EnhancedDataAnalyzer) IsLangChainEnabled() bool {
	return e.useLangChain
}

// AnalyzeWithLangChain performs analysis using LangChain directly
func (e *EnhancedDataAnalyzer) AnalyzeWithLangChain(ctx context.Context, query, timeFrame string) (*LangChainResult, error) {
	return e.langchain.AnalyzeWithLangChain(ctx, query, timeFrame)
}
