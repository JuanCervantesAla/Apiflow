The GPT and Claude node implementations (gpt_node.go, claude_node.go) were removed from this project because external policies forbid using those providers directly.

All AI functionality is now provided via the Groq-backed node type `groq`, which uses the GROQ_API_KEY and Groq's OpenAI-compatible API.

If you need to reintroduce additional AI providers in the future, add new node implementations instead of reusing the old ones.