import { Navbar } from "@/components/navbar"
import { Hero } from "@/components/hero"
import { Stats } from "@/components/stats"
import { CodeShowcase } from "@/components/code-showcase"
import { Features } from "@/components/features"
import { SkillFormat } from "@/components/skill-format"
import { Comparison } from "@/components/comparison"
import { Security } from "@/components/security"
import { Architecture } from "@/components/architecture"
import { CTA } from "@/components/cta"
import { Footer } from "@/components/footer"

export default function HomePage() {
  return (
    <main className="min-h-screen">
      <Navbar />
      <Hero />
      <Stats />
      <CodeShowcase />
      <SkillFormat />
      <Features />
      <Comparison />
      <Security />
      <Architecture />
      <CTA />
      <Footer />
    </main>
  )
}
