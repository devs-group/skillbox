import Link from "next/link"
import { Github } from "lucide-react"

export function Footer() {
  return (
    <footer className="border-t border-border py-12">
      <div className="mx-auto max-w-6xl px-6">
        <div className="flex flex-col md:flex-row items-center justify-between gap-6">
          <div className="flex items-center gap-6">
            <Link href="/" className="flex items-center gap-2">
              <div className="flex items-center justify-center w-6 h-6 rounded bg-primary">
                <span className="text-primary-foreground font-mono font-bold text-xs">S</span>
              </div>
              <span className="font-mono font-bold text-sm text-foreground">skillbox</span>
            </Link>
            <span className="text-sm text-muted-foreground">
              Built by{" "}
              <Link href="https://devs-group.com" target="_blank" rel="noopener noreferrer" className="text-foreground hover:text-primary transition-colors underline underline-offset-4">
                devs group
              </Link>
              {" + "}
              <Link href="https://www.codify.ch/en" target="_blank" rel="noopener noreferrer" className="text-foreground hover:text-primary transition-colors underline underline-offset-4">
                codify
              </Link>
              {" "}in Switzerland
            </span>
          </div>

          <div className="flex items-center gap-6">
            <Link
              href="https://github.com/devs-group/skillbox"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              <Github className="w-4 h-4" />
              GitHub
            </Link>
            <Link
              href="https://github.com/alibaba/OpenSandbox"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              OpenSandbox
            </Link>
            <Link
              href="https://github.com/devs-group/skillbox/blob/main/LICENSE"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              MIT License
            </Link>
            <Link
              href="https://github.com/devs-group/skillbox/blob/main/CONTRIBUTING.md"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm text-muted-foreground hover:text-foreground transition-colors"
            >
              Contributing
            </Link>
          </div>
        </div>
      </div>
    </footer>
  )
}
