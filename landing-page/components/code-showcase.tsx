"use client"

import { useState } from "react"

const tabs = [
  {
    label: "Python",
    code: `from skillbox import Client

client = Client("http://localhost:8080", "sk-your-key")

# Discover what skills are available
for skill in client.list_skills():
    print(f"{skill.name}: {skill.description}")

# Run a skill — structured in, structured out
result = client.run("data-analysis", input={
    "data": [1, 2, 3, 4, 5]
})
print(result.output)
# {"row_count": 5, "mean": 3.0, ...}`,
    filename: "agent.py",
  },
  {
    label: "Go",
    code: `import skillbox "github.com/devs-group/skillbox/sdks/go"

client := skillbox.New(
    "http://localhost:8080",
    "sk-your-key",
    skillbox.WithTenant("my-team"),
)

result, err := client.Run(ctx, skillbox.RunRequest{
    Skill: "text-summary",
    Input: json.RawMessage(\`{
        "text": "Long text here...",
        "max_sentences": 3
    }\`),
})

if result.HasFiles() {
    err = client.DownloadFiles(ctx, result, "./output")
}`,
    filename: "main.go",
  },
  {
    label: "cURL",
    code: `curl -s http://localhost:8080/v1/executions \\
  -H "Authorization: Bearer $SKILLBOX_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "skill": "data-analysis",
    "input": {
      "data": [
        {"name": "Alice", "age": 30},
        {"name": "Bob", "age": 25}
      ]
    }
  }' | jq .

# Response:
# {
#   "id": "exec-abc-123",
#   "status": "completed",
#   "output": {"row_count": 2, "mean_age": 27.5},
#   "files": ["report.pdf"]
# }`,
    filename: "terminal",
  },
  {
    label: "LangChain",
    code: `from langchain_anthropic import ChatAnthropic
from langgraph.prebuilt import create_react_agent

# Build tools from all registered skills
tools = build_skillbox_toolkit(
    "http://localhost:8080",
    "sk-your-key"
)

# Agent sees tools like skillbox_data_analysis,
# reads their descriptions, picks the right one,
# calls it with structured input,
# gets structured output
agent = create_react_agent(
    ChatAnthropic(model="claude-sonnet-4-6"),
    tools
)

result = agent.invoke({
    "messages": [{
        "role": "user",
        "content": "Analyze this data: name,age\\nAlice,30\\nBob,25"
    }]
})`,
    filename: "agent_langchain.py",
  },
]

function highlightSyntax(code: string, lang: string) {
  // Simple syntax highlighting
  const lines = code.split("\n")
  return lines.map((line, i) => {
    let highlighted = line
      // Strings
      .replace(/(["'`])(.*?)\1/g, '<span class="text-primary">$1$2$1</span>')
      // Comments
      .replace(/(#.*$)/gm, '<span class="text-muted-foreground/60 italic">$1</span>')
      // Keywords
      .replace(/\b(from|import|for|in|if|print|def|class|return|const|let|var|await|async|func|err|ctx)\b/g,
        '<span class="text-foreground font-medium">$1</span>')

    if (lang === "cURL") {
      highlighted = line
        .replace(/(curl|jq)/g, '<span class="text-foreground font-medium">$1</span>')
        .replace(/(#.*$)/gm, '<span class="text-muted-foreground/60 italic">$1</span>')
        .replace(/(["'])(.*?)\1/g, '<span class="text-primary">$1$2$1</span>')
    }

    return (
      <div key={i} className="flex">
        <span className="inline-block w-8 text-right mr-4 text-muted-foreground/30 select-none text-xs leading-6">{i + 1}</span>
        <span className="leading-6" dangerouslySetInnerHTML={{ __html: highlighted }} />
      </div>
    )
  })
}

export function CodeShowcase() {
  const [activeTab, setActiveTab] = useState(0)

  return (
    <section id="how-it-works" className="py-20 md:py-32">
      <div className="mx-auto max-w-6xl px-6">
        <div className="text-center mb-12">
          <p className="font-mono text-sm text-primary mb-3 tracking-wider uppercase">Integration</p>
          <h2 className="text-3xl md:text-5xl font-bold text-foreground text-balance">
            A few lines in your code
          </h2>
          <p className="mt-4 text-lg text-muted-foreground max-w-2xl mx-auto text-pretty">
            Zero-dependency SDKs for Go and Python. Or just use the REST API directly.
          </p>
        </div>

        <div className="rounded-lg border border-border bg-card overflow-hidden">
          {/* Tabs */}
          <div className="flex items-center border-b border-border overflow-x-auto">
            {tabs.map((tab, i) => (
              <button
                key={tab.label}
                onClick={() => setActiveTab(i)}
                className={`px-5 py-3 text-sm font-mono transition-colors whitespace-nowrap ${
                  activeTab === i
                    ? "text-primary border-b-2 border-primary bg-secondary/30"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                {tab.label}
              </button>
            ))}
            <div className="flex-1" />
            <span className="px-4 py-3 text-xs font-mono text-muted-foreground/50">
              {tabs[activeTab].filename}
            </span>
          </div>

          {/* Code */}
          <div className="p-5 font-mono text-sm overflow-x-auto">
            <pre className="text-muted-foreground">
              {highlightSyntax(tabs[activeTab].code, tabs[activeTab].label)}
            </pre>
          </div>
        </div>
      </div>
    </section>
  )
}
