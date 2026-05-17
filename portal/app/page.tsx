"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";

interface Card {
  id: string;
  tag: string;
  title: string;
  desc: string;
  cmd: string;
}

const CARDS: Card[] = [
  {
    id: "s3",
    tag: "STORAGE",
    title: "S3 BUCKET",
    desc: "Versioned object storage with AES-256 encryption at rest.",
    cmd: "> terrasense create s3-bucket",
  },
  {
    id: "ec2",
    tag: "COMPUTE",
    title: "EC2 INSTANCE",
    desc: "t3.medium with IAM role, security group, and auto-scaling.",
    cmd: "> terrasense create ec2-instance",
  },
  {
    id: "rds",
    tag: "DATABASE",
    title: "RDS POSTGRESQL",
    desc: "Multi-AZ managed PostgreSQL 15 with automated backups.",
    cmd: "> terrasense create rds-postgres",
  },
  {
    id: "vpc",
    tag: "NETWORKING",
    title: "VPC NETWORK",
    desc: "Public/private subnets across 3 AZs with NAT gateway.",
    cmd: "> terrasense create vpc",
  },
  {
    id: "eks",
    tag: "KUBERNETES",
    title: "EKS CLUSTER",
    desc: "Managed Kubernetes 1.29 with IRSA and cluster autoscaler.",
    cmd: "> terrasense create eks-cluster",
  },
  {
    id: "iam",
    tag: "SECURITY",
    title: "IAM ROLE",
    desc: "Least-privilege IAM role with inline policy and instance profile.",
    cmd: "> terrasense create iam-role",
  },
];

const S = {
  bg: "#0A0A0A",
  surface: "#111111",
  border: "#222222",
  text: "#E5E5E5",
  muted: "#555555",
  dim: "#333333",
  accent: "#6366F1",
  white: "#FFFFFF",
} as const;

function HR() {
  return <div style={{ borderTop: `1px solid ${S.border}` }} />;
}

function LandingCard({ card, onClick }: { card: Card; onClick: (id: string) => void }) {
  const [hover, setHover] = useState(false);
  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => onClick(card.id)}
      onKeyDown={(e) => e.key === "Enter" && onClick(card.id)}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      style={{
        border: `1px solid ${hover ? "#444444" : S.border}`,
        padding: "24px",
        cursor: "pointer",
        display: "flex",
        flexDirection: "column",
        gap: 16,
        height: "100%",
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
        <span style={{ fontSize: 15, fontWeight: 700, letterSpacing: "0.02em", color: S.white }}>
          {card.title}
        </span>
        <span style={{ fontSize: 10, fontWeight: 700, letterSpacing: "0.12em", color: S.muted, paddingTop: 2 }}>
          {card.tag}
        </span>
      </div>

      <p style={{ fontSize: 12, color: S.muted, lineHeight: 1.7, flexGrow: 1 }}>
        {card.desc}
      </p>

      <div style={{ borderTop: `1px solid ${S.border}`, paddingTop: 14 }}>
        <span style={{ fontSize: 11, color: hover ? S.accent : S.dim }}>
          {card.cmd}
        </span>
      </div>
    </div>
  );
}

export default function Home() {
  const router = useRouter();

  function handleCardClick(id: string) {
    router.push(`/changes/${id}`);
  }

  const fontMono = "var(--font-jetbrains-mono), 'Courier New', monospace";
  const fontInter = "var(--font-inter), sans-serif";

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        flexDirection: "column",
        background: S.bg,
        color: S.text,
        fontFamily: fontMono,
        fontSize: 13,
        lineHeight: 1.6,
      }}
    >
      {/* Nav */}
      <nav
        style={{
          padding: "20px 32px",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          borderBottom: `1px solid ${S.border}`,
        }}
      >
        <span style={{ fontFamily: fontInter, fontWeight: 900, fontSize: 15, letterSpacing: "0.18em", color: S.white }}>
          TERRASENSE
        </span>
        <div style={{ display: "flex", gap: 28 }}>
          <a href="#contact" style={{ fontSize: 11, letterSpacing: "0.1em", color: S.muted, textDecoration: "none" }}>
            CONTACT
          </a>
          <a
            href="https://github.com/hashir-zahoor-kh/TerraSense"
            target="_blank"
            rel="noreferrer"
            style={{ fontSize: 11, letterSpacing: "0.1em", color: S.muted, textDecoration: "none" }}
          >
            GITHUB
          </a>
        </div>
      </nav>

      {/* Hero */}
      <section style={{ padding: "80px 32px 72px", maxWidth: 1200, width: "100%", margin: "0 auto" }}>
        <h1
          style={{
            fontFamily: fontInter,
            fontWeight: 900,
            fontSize: "clamp(56px, 9vw, 120px)",
            letterSpacing: "-3px",
            lineHeight: 0.95,
            color: S.white,
            marginBottom: 32,
          }}
        >
          TERRASENSE
        </h1>
        <p style={{ fontSize: 13, color: S.muted, maxWidth: 480, lineHeight: 1.7 }}>
          Describe infrastructure in plain English. An LLM generates Terraform HCL, a correction
          loop fixes security failures, and a human approves before anything touches production.
        </p>
      </section>

      <HR />

      {/* Cards */}
      <section
        style={{ padding: "48px 32px", maxWidth: 1200, width: "100%", margin: "0 auto", flexGrow: 1 }}
      >
        <div
          style={{
            marginBottom: 28,
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
          }}
        >
          <span style={{ fontSize: 11, letterSpacing: "0.12em", color: S.muted }}>SELECT TEMPLATE</span>
          <span style={{ fontSize: 11, color: S.dim }}>6 templates</span>
        </div>
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(300px, 1fr))",
            gap: 0,
            border: `1px solid ${S.border}`,
          }}
        >
          {CARDS.map((card) => (
            <div
              key={card.id}
              style={{
                borderRight: `1px solid ${S.border}`,
                borderBottom: `1px solid ${S.border}`,
                marginRight: -1,
                marginBottom: -1,
              }}
            >
              <LandingCard card={card} onClick={handleCardClick} />
            </div>
          ))}
        </div>
      </section>

      <HR />

      {/* Contact */}
      <section
        id="contact"
        style={{ padding: "64px 32px", maxWidth: 1200, width: "100%", margin: "0 auto" }}
      >
        <div style={{ border: `1px solid ${S.border}`, padding: "48px" }}>
          <div style={{ maxWidth: 560 }}>
            <h2
              style={{
                fontFamily: fontInter,
                fontWeight: 900,
                fontSize: "clamp(22px, 3vw, 32px)",
                letterSpacing: "-0.5px",
                color: S.white,
                marginBottom: 16,
              }}
            >
              INTERESTED IN DEPLOYING THIS?
            </h2>
            <p style={{ fontSize: 13, color: S.muted, lineHeight: 1.8, marginBottom: 32 }}>
              If you&apos;d like TerraSense deployed for your organization, reach out directly.
            </p>
            <a
              href="mailto:hashirzahoor74@icloud.com"
              style={{
                display: "inline-block",
                border: `1px solid ${S.accent}`,
                color: S.accent,
                padding: "10px 24px",
                fontSize: 12,
                fontWeight: 700,
                letterSpacing: "0.08em",
                fontFamily: fontMono,
                textDecoration: "none",
              }}
            >
              hashirzahoor74@icloud.com
            </a>
          </div>
        </div>
      </section>

      <HR />

      <footer
        style={{
          padding: "20px 32px",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <span style={{ fontSize: 10, color: S.dim, letterSpacing: "0.08em" }}>TERRASENSE</span>
        <span style={{ fontSize: 10, color: S.dim }}>
          Go · Next.js · Anthropic · Terraform · Checkov
        </span>
      </footer>
    </div>
  );
}
