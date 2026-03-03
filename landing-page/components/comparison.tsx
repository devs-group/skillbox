import { Check, X, Minus } from "lucide-react"

type CellValue = "yes" | "no" | "partial" | string

const categories = [
  { label: "Self-hosted", skillbox: "yes" as CellValue, e2b: "Experimental" as CellValue, modal: "no" as CellValue, daytona: "partial" as CellValue },
  { label: "Skill catalog", skillbox: "yes" as CellValue, e2b: "no" as CellValue, modal: "no" as CellValue, daytona: "no" as CellValue },
  { label: "Structured I/O", skillbox: "JSON in/out" as CellValue, e2b: "Raw stdout" as CellValue, modal: "Raw stdout" as CellValue, daytona: "Raw stdout" as CellValue },
  { label: "Agent introspection", skillbox: "yes" as CellValue, e2b: "no" as CellValue, modal: "no" as CellValue, daytona: "no" as CellValue },
  { label: "LangChain-native", skillbox: "yes" as CellValue, e2b: "Manual" as CellValue, modal: "Manual" as CellValue, daytona: "Manual" as CellValue },
  { label: "Network disabled", skillbox: "Always" as CellValue, e2b: "Optional" as CellValue, modal: "no" as CellValue, daytona: "no" as CellValue },
  { label: "Zero-dep SDK", skillbox: "yes" as CellValue, e2b: "no" as CellValue, modal: "no" as CellValue, daytona: "no" as CellValue },
  { label: "File management", skillbox: "yes" as CellValue, e2b: "partial" as CellValue, modal: "no" as CellValue, daytona: "no" as CellValue },
  { label: "License", skillbox: "MIT" as CellValue, e2b: "Apache-2.0" as CellValue, modal: "Proprietary" as CellValue, daytona: "Apache-2.0" as CellValue },
]

function CellContent({ value }: { value: CellValue }) {
  if (value === "yes") return <Check className="w-4 h-4 text-primary" />
  if (value === "no") return <X className="w-4 h-4 text-muted-foreground/40" />
  if (value === "partial") return <Minus className="w-4 h-4 text-muted-foreground/60" />
  return <span className="text-xs">{value}</span>
}

export function Comparison() {
  return (
    <section id="comparison" className="py-20 md:py-32 border-t border-border">
      <div className="mx-auto max-w-6xl px-6">
        <div className="text-center mb-16">
          <p className="font-mono text-sm text-primary mb-3 tracking-wider uppercase">Comparison</p>
          <h2 className="text-3xl md:text-5xl font-bold text-foreground text-balance">
            How Skillbox stacks up
          </h2>
          <p className="mt-4 text-lg text-muted-foreground max-w-2xl mx-auto text-pretty">
            Cloud-only is fine until it isn{"'"}t. When your data can{"'"}t leave your network, Skillbox is the answer.
          </p>
        </div>

        <div className="overflow-x-auto rounded-lg border border-border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-secondary/30">
                <th className="text-left px-6 py-4 font-medium text-muted-foreground" />
                <th className="text-center px-6 py-4 font-mono font-bold text-primary">Skillbox</th>
                <th className="text-center px-6 py-4 font-mono font-medium text-muted-foreground">E2B</th>
                <th className="text-center px-6 py-4 font-mono font-medium text-muted-foreground">Modal</th>
                <th className="text-center px-6 py-4 font-mono font-medium text-muted-foreground">Daytona</th>
              </tr>
            </thead>
            <tbody>
              {categories.map((row, i) => (
                <tr key={row.label} className={`border-b border-border ${i % 2 === 0 ? "bg-card" : "bg-secondary/10"}`}>
                  <td className="px-6 py-4 font-medium text-foreground whitespace-nowrap">{row.label}</td>
                  <td className="px-6 py-4 text-center">
                    <div className="flex items-center justify-center">
                      <CellContent value={row.skillbox} />
                    </div>
                  </td>
                  <td className="px-6 py-4 text-center text-muted-foreground">
                    <div className="flex items-center justify-center">
                      <CellContent value={row.e2b} />
                    </div>
                  </td>
                  <td className="px-6 py-4 text-center text-muted-foreground">
                    <div className="flex items-center justify-center">
                      <CellContent value={row.modal} />
                    </div>
                  </td>
                  <td className="px-6 py-4 text-center text-muted-foreground">
                    <div className="flex items-center justify-center">
                      <CellContent value={row.daytona} />
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  )
}
