> **New here?** Start with [`docs/GETTING_STARTED.md`](docs/GETTING_STARTED.md) — a first-hour guide for human developers (prerequisites, build, project structure, IDE setup, CI/CD, and making your first PR).

# Local CI Validation (MANDATORY)

All contributors must use the provided Makefile and actionlint to validate their changes locally before pushing or opening a PR.

## How to Validate Locally

1. **Run all checks:**
  ```bash
  make test
  ```
2. **Rust workspace (mirrors Workspace Rust Checks CI):**
  ```bash
  make lci
  # Alias:
  make local-ci
  # If stevedores-org/local-ci CLI is installed, `make local-ci`
  # uses `.local-ci.toml`; otherwise it falls back to tools/lci.
  # Machine-readable output:
  make lci-json
  ```
3. **Just check YAML workflows:**
  ```bash
  make test-yaml
  ```

If you do not have actionlint installed:
```bash
brew install actionlint
```

For more details, see [docs/LOCAL_CI_VALIDATION.md](docs/LOCAL_CI_VALIDATION.md).

This workflow ensures that all documentation and YAML workflows are robust and error-free before merging to develop.

## Agent Pre-Commit

All agents must run the repo pre-commit checks before committing changes.
# **Grow Without Limits — Lornuai, Inc.**

## ---

[![Bun](https://img.shields.io/badge/Bun-1.3+-black?logo=bun&logoColor=white)](https://bun.sh)
[![Rust](https://img.shields.io/badge/Rust-1.80+-CE412B?logo=rust&logoColor=white)](https://www.rust-lang.org/)
[![Crossplane](https://img.shields.io/badge/Crossplane-Control%20Plane-0470FF?logo=crossplane&logoColor=white)](https://crossplane.io)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-EKS%20%2F%20GKE-326CE5?logo=kubernetes&logoColor=white)](https://kubernetes.io)
[![Kustomize](https://img.shields.io/badge/Kustomize-v5+-1ABC9C?logo=kubernetes&logoColor=white)](https://kustomize.io)


**Mission**

**Deliver a 50–70% reduction in engineering time and operational overhead** through a hardened, metadata-driven **Minimum Viable Infrastructure (MVI)**. Grow Without Limits consolidates delivery onto **one EKS/GKE cluster** with **Kustomize-based multi-namespace isolation** across `lornu-ai-dev`, `lornu-ai-staging`, and `lornu-ai-prod` namespaces.

## **Autonomous Workflow Protocol**

When tasked with project maintenance or objective progression, follow this Algorithmic Logic loop:

1. **Poll & Prioritize**:
   - Use available tools to fetch the latest open GitHub Pull Requests and Issues.
   - Categorize by impact and urgency.

2. **Analyze & Engage**:
   - Review each item thoroughly.
   - Post relevant comments, code reviews, or technical feedback directly to the threads.

3. **Document**:
   - Update the primary GitHub Issue(s) with a concise summary of progress made and the current status of the objective.
   - This ensures the "Source of Truth" is always current.

4. **Iterate**:
   - Identify the next highest-priority action item to maintain momentum.
   - Propose or initiate the next logical file change or command.

5. **Exception Handling**:
   - If a task is blocked, requires credentials you lack, or needs human architectural input, **STOP** immediately.
   - Clearly define the blocker and ask the user for guidance.
## **Shared Agent State**

Lornu AI utilizes a **Shared State Mechanism** to synchronize context across multiple IDE agents (Windsurf, Cursor, VSCode):

- **File**: `.lornu/STATE.json` (git-ignored)
- **Purpose**: Tracks active agents, tasks, branch status, and compliance.
- **Protocol**: Managed by `lornu-mcp-hub`, exposed via `mcp://lornu-ai/state`.
- **Security**: Local-only, file permissions 600.

## **CI/CD Quality Gates (Grow Without Limits Unified)**

The repository implements a 3-tier validation engine for all infrastructure changes:

1. **Level 1: Syntactic Build Test**: Validates that all Kustomize overlays can be built successfully (`kustomize build`).
2. **Level 2: Schema Validation**: Uses `kubeconform` to validate Kubernetes resources against official schemas.
3. **Level 3: Security & Policy Guardrails**: Uses `Checkov` to scan for security misconfigurations and policy violations.
4. **Secret Scanning**: Uses `Gitleaks` to prevent sensitive data from being committed.
5. **Label Compliance**: Ensures all resources have mandatory `lornu.ai/environment`, `lornu.ai/managed-by`, and `lornu.ai/asset-id` labels.
6. **Gemini Code Review**: Automated AI code review for all Pull Requests (Issue #695).
7. **PR Merge Policy**: NO pull requests may be merged if there are failing CI checks or unresolved merge conflicts.
8. **Unit Test Coverage**: Coverage is enforced with a phased rollout via `.github/workflows/rust-coverage.yml` (75% threshold: warn on PRs/non-main pushes, block on `main` push).

**Declarative GitOps Policy**: All infrastructure changes must follow the declarative GitOps workflow. Imperative commands are prohibited. See [docs/DECLARATIVE_GITOPS_POLICY.md](docs/DECLARATIVE_GITOPS_POLICY.md).

## **Zero-Trust OIDC Authentication**

Lornu AI implements **universal OIDC federation** for sovereign, zero-secret authentication across all environments:

### **AWS Hub (EKS) - IRSA**
- **IAM OIDC Provider**: GitHub Actions + EKS service accounts
- **Roles**: `lornu-ai-github-actions-role`, `lornu-ai-flux-controller-role`
- **Annotation**: `eks.amazonaws.com/role-arn: arn:aws:iam::${AWS_ACCOUNT_ID}:role/...`

### **GCP Spoke (GKE) - Workload Identity**
- **Workload Identity Pool**: `github-pool` with GitHub + EKS providers
- **Service Account**: `lornu-ai-sa@${GCP_PROJECT_ID}.iam.gserviceaccount.com`
- **Annotation**: `iam.gke.io/gcp-service-account: ...`

### **GitOps Flow**
1. **GitHub Actions** → OIDC → IAM Roles → Temporary Credentials
2. **EKS/GKE Pods** → IRSA/Workload Identity → Cloud Resources
3. **Zero Static Secrets** - All authentication is federated

These gates are enforced on every Pull Request to `develop` and `staging`.

## **Global Low-Cost Data Fabric (GLCDF)**

The **GLCDF** is a unified, globally distributed persistence layer designed for autonomous agents, optimized for extreme cost-efficiency and sovereign data control (Issue #541).

- **Storage Engine**: Azure Cosmos DB (Serverless/Provisioned Multi-region) + Azure Blob Storage.
- **Architecture**: Decoupled compute/storage with zero-copy ingest and local-first caching.
- **Cost Optimization**: 50-70% reduction in state storage costs compared to traditional NoSQL.
- **Latency Sovereignty**: Global distribution ensuring <100ms state retrieval for agents worldwide.
- **Governance**: Integrated with `warden` for PII/Secret scrubbing and `zen` for governance.
- **Client Library**: `packages/glcdf-client/` - Python async client with lazy loading and local caching.

### Quick Start (glcdf-client)
Client libraries available in Rust and TypeScript for agent state management.

- **Build Once, Promote Often**: All container images are built and mirrored to all clouds by Dockworker.ai, ensuring a single cryptographic digest. Production is promoted by SHA.
- **Target Branch**: All development PRs must target the `develop` branch.
- `.github/workflows/`: Consolidated intelligent orchestrators (Issue #440)
- `dockworker.toml`: Sovereign Build Manifest (Issue #439)
- `crossplane/hub/deploy/overlays/gcp/`: GCP hub control-plane + infra (PRIMARY Operational Hub)
- `crossplane/hub/deploy/overlays/azure/`: Azure hub control-plane + infra (In Provisioning)
- **Production**: Updates to `main` are for production stabilization and promotion.

* **Control Plane (Private Hub)**: `private-lornu-ai` (Zero-Secret, SOPS-Encrypted).
* **Workload Plane (Public Spoke)**: `lornu-ai` (Application Manifests, Open-Source compatible).
* **Sync Mechanism**: Flux CD bootstrapped to `private-lornu-ai`.
* **Single EKS/GKE cluster** with `lornu-ai-dev`, `lornu-ai-staging`, and `lornu-ai-prod` namespaces.
* **Kustomize overlays** per environment (Spoke-managed).
* **Crossplane** as infrastructure control plane.
* **Modern runtimes**: Rust (Backend/Core), Bun (Frontend).
* **Tooling**: Rust (MANDATORY - Rust-or-Bust policy for all backend components), Bun (Frontend), Infrastructure Cleanup (`scripts/aws-cleanup.sh`).
* **Performance Sovereignty**: Bolt ⚡ persona ensures sub-100ms latency and maximum cost-efficiency.
* **Unified Organization**: High-value AI/Agentic repositories consolidated from legacy orgs (see [Repository Migration Audit](docs/STEVEI101_REPO_MIGRATION_AUDIT.md)).
* **Global Low-Cost Data Fabric (GLCDF)**: Multi-region, Azure-optimized persistence layer for sovereign agent state and long-term memory (Issue #541).

## **Key Operational Agents**

Lornu AI includes several autonomous agents for operational excellence:

- **SRE Auto-Remediation Agent** (`ai-agents/sre-agent-rs/`): Rust-based Kubernetes pod failure detection with AI-driven remediation using GPT-4o via Rig framework
- **Drift Enforcer Agent** (`ai-agents/drift-enforcer/`): Multi-cloud resource sprawl detection and automated decommissioning
- **Workflow Sentinel** (`ai-agents/workflow-sentinel-rs/`): High-performance GitHub Actions linter implemented in Rust
- **PR Review Agent** (`ai-agents/ai-agent-pr-review/`): Automated code review with Gemini integration
- **Cloudflare DNS Agent** (`ai-agents/cloudflare-dns-agent/`): Multi-cloud DNS synchronization across AWS Route53, GCP Cloud DNS, and Azure DNS

For complete agent documentation, see [AGENTS.md](AGENTS.md).

## **Directory Structure**

```plaintext
crossplane/hub/deploy/          # Unified hub infrastructure (Issue #472)
  ├── base/                     # Shared infrastructure APIs
  └── overlays/{aws,azure,gcp}/ # Cloud-specific + app-agents
crossplane/spoke/apps/          # Spoke workloads (23 apps + 5 hub agents)
apps/                           # Deployable applications (23 total)
  ├── automation-hub/           # Cloudflare Workers (OIDC hub + CI automation)
  ├── lornu-ai/                 # Lornu AI application parent
  │   ├── frontend/             # React frontend (Bun)
  │   └── deploy/               # Deployment manifests (Flux/Kustomize)
  ├── dockworker-ai/            # Dockworker AI application parent
  │   ├── frontend/             # React frontend (Bun)
  │   ├── backend/              # Rust backend
  │   └── worker/               # Rust worker
  ├── api/                      # Rust backend
  ├── cloudflare-dns-agent/     # DNS Sync Agent (Rust)
  ├── cloudflare-edge-discovery/ # Edge Discovery Worker (TypeScript)
  └── [17 more applications]
ai-agents/                      # Autonomous Agents (Rust)
  ├── aiops-agent/              # AIOps Auto-Remediation
  ├── ai-agent-cleaner/         # Security scanner
  ├── ai-agent-pr-review/       # CI gatekeeper
  ├── sre-agent/                # SRE Auto-Remediation
  └── agent-registry/           # FastA2A Agent Registry Service
packages/                       # Reusable libraries ONLY
  ├── glcdf-client/             # Global Low-Cost Data Fabric client (Rust/TS)
  ├── lornu-mcp-hub/            # MCP tools hub
  └── ai-agent-cleaner/         # Cleaner library
templates/                      # Satellite repository templates (7-File Rule, CI/CD)
terraform/                      # Legacy infrastructure (Terraform Cloud)
```

### **Lornu AI Multi-Cloud Mesh (Active Satellites)**
| Satellite | Role | Primary Cloud | Status |
| :--- | :--- | :--- | :--- |
| **gitlab-container-builds** | Sovereign Agent Builder & CI Templates | GitLab | ✅ Active |
| **automation-hub** | Centralized Cloudflare Workers | Cloudflare | ✅ Active |
| **lornu-aks-hub** | Azure Hub Control Plane | Azure | ✅ Active |

| Satellite | Cloud | Region | Endpoint | Status | Logistcs |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **lornu-aks-hub** | Azure | eastus2 | `api.lornu.ai` | ✅ Active (Hub) | Flux/Crossplane |
| **<GCP_GKE_CLUSTER_NAME>** | GCP | us-central1 | `<GCP_GKE_LB_IP>` | ✅ Active (GCP) | Flux/GKE |
| **dockworker-ai** | AWS | us-east-2 | `dockworker.lornu.ai` | ✅ Active | Container packaging platform |
| **financial-agent-spoke** | GCP | us-central1 | `email.lornu.ai` | ⚠️ Migrating | App+SQL Overlay |
| **ai-agent-core** | AWS | us-east-2 | `core.lornu.ai` | ✅ Active | Flux/Crossplane |
| **lornu-crawler** | GCP | us-central1 | `crawler.lornu.ai` | ✅ Active | Playwright/Flux |
| **YouTube** | N/A | N/A | `youtube.com/@lornu-ai` | ✅ Active | Community Link |

## **Satellite Repositories**

Lornu AI uses a **hub-and-spoke architecture** where application source code lives in separate **satellite repositories**, while this hub repository manages deployment manifests and infrastructure:

### **Active Satellite Repositories**

| Repository | Purpose | Registry | Status |
|------------|---------|----------|--------|
| logistics-supervisor-api | Container packaging platform | AWS ECR | ✅ Active |
| financial-agent-spoke | Intelligent email response | AWS ECR | ✅ Active |
| [ai-agents-core](https://github.com/lornu-ai/ai-agents-core) | Centralized agent framework | AWS ECR | ✅ Active |
| [lornu-crawler](https://github.com/lornu-ai/lornu-crawler) | Site crawler & screenshotter | GCP Artifact Registry | ✅ Active |
| [cloudflare-dns-agent](apps/cloudflare-dns-agent/README.md) | Multi-Cloud DNS Sync Agent | ECR/GAR/ACR | ✅ Active |
| [cloudflare-edge-discovery](apps/cloudflare-edge-discovery/README.md) | Edge Discovery Worker (Hyperdrive) | Cloudflare Workers | ✅ Active |
| [lornu.ai](https://github.com/lornu-ai/lornu.ai) | Hub repository (Crossplane/Flux) | AWS ECR | ✅ Active |
| [agentnav](https://github.com/lornu-ai/agentnav) | Agentic Navigation module | N/A | ✅ Active |
| [product-mindset](https://github.com/lornu-ai/product-mindset) | Product Mindset analytics | N/A | ✅ Active |
| [cursor-agent-rules](https://github.com/lornu-ai/cursor-agent-rules) | Automated AI-Code Review | N/A | ✅ Active |
| [autonomous-sre-agent](https://github.com/lornu-ai/autonomous-sre-agent) | Sovereign SRE drift detection + auto-remediation | N/A | ✅ Active |
| [gcp-conversation-service](https://github.com/lornu-ai/gcp-conversation-service) | Gemini Integration hub | GCP Cloud Run | ✅ Active |
| [email-triage](https://github.com/lornu-ai/email-triage) | Automated email triage | N/A | ✅ Active |
| [antigravity-ide-tracker](https://github.com/lornu-ai/antigravity-ide-tracker) | IDE/Dev workflow tracking | N/A | ✅ Active |
| [not-sure](https://github.com/lornu-ai/not-sure) | Migration of stevei101/not-sure | N/A | ✅ Active |

### **Setting Up New Satellites**

All container builds and multi-cloud pushes are now handled by Dockworker.ai, ensuring a single cryptographic digest across AWS, GCP, and Azure. See `dockworker.toml` for configuration and `.github/workflows/dockworker-sync.yml` for the workflow.

**Org policy — never `Dockerfile`:** lornu-ai and stevedores-org build images with **Nix OCI** (`flake.nix`) + **dockworker.ai**, not hand-written Dockerfiles. Do not add or restore `Dockerfile` / `docker build`; see the 6-file instruction set (`.cursorrules`, `AGENTS.md`, etc.) for agents.

### **Automation & Infrastructure Satellites**

**Cloudflare Workers** are managed in the **automation-hub** repository:
- **Repository**: [lornu-ai/automation-hub](https://github.com/lornu-ai/automation-hub)
- **Workers**: `automation-hub` (CI automation), `lornu-edge-discovery` (agent discovery via Hyperdrive)
- **Private RAG (Issue #565)**: R2 bucket + Vectorize for agent knowledge grounding
  - Endpoints: `/api/rag/search`, `/api/vectorize/trigger`, `/api/vectorize/status`
  - R2 Bucket: `lornu-private-rag-prod`
  - Vectorize Index: `lornu-private-knowledge` (768 dims, bge-base-en-v1.5)
- **Deployment**: Uses Wrangler CLI (`wrangler deploy`)
- **Related Issues**: #589 (Edge Discovery), #542 (CI Automation), #565 (Private RAG)

**GitLab CI/CD** configurations are managed in the **gitlab-container-builds** repository:
- **Repository**: [lornu-ai/gitlab-container-builds](https://github.com/lornu-ai/gitlab-container-builds)
- **Purpose**: GitLab CI/CD pipelines for container builds and multi-cloud mirroring
- **Related**: GitLab pull mirroring setup (see `docs/GITLAB_MIRROR_SETUP.md`)

## **Development Standards**

### **The 7-File Rule (Documentation as Infrastructure)**

Treating documentation as infrastructure ensures all AI assistants (Cursor, Windsurf, Copilot) operate with synchronized context, preventing schema drift and maintaining the **Sovereign Knowledge Fabric.**

**Mandatory Context Files:**
1. **`.cursorrules`**: Local IDE rules.
2. **`AGENTS.md`**: A2A Protocol and Agent capabilities.
3. **`CLAUDE.md`**: Build/test commands and project context.
4. **`README.md`**: High-level project map.
5. **`ARCH_PRESERVE.md`**: Architectural preservation rules.
6. **`.github/copilot-instructions.md`**: GitHub Copilot context.
7. **`.github/system-instruction.md`**: Global "Sovereign Intelligence" standards.

**When to update:**
- New features added to `apps/` or `app-agents/`
- Changes to deployment patterns, build commands, or infrastructure
- New standards, conventions, or governance policies

### **Agent Workflow (Autonomous Operation Loop)**

When working autonomously, agents follow this algorithmic loop:

1. **Poll & Prioritize**: Fetch the latest open GitHub Pull Requests and Issues.
2. **Analyze & Engage**: Review each item and post relevant comments or actionable feedback.
3. **Document**: Update the primary GitHub Issue(s) with a concise summary of progress made and current status.
4. **Iterate**: Identify the next highest-priority action item to maintain momentum.
5. **Exception Handling**: If a task is blocked or requires human input, stop and ask for guidance immediately.

### **Pull Request Requirements**

* **Labeling**: Apply a label for the **worker/agent** (e.g., `codex`, `claude`, `antigravity`, `vs-code`, `jules`, `gemini-cli`).
  - Create label if needed: `gh label create <agent-name>`
  - Apply to PR: `gh pr edit <pr-number> --add-label <agent-name>`
* **Base Branch**: All PRs must target the `develop` branch.
* **Documentation Sync**: Every PR must include updates to the **7-File Rule** if architectural changes are made.
* **Terraform Hygiene**: Run `terraform fmt` and `terraform validate` before pushing changes.

## **Infrastructure & Deployment**

### **GitOps Architecture (Flux + ArgoCD + Crossplane + ESO)**

<<<<<<< HEAD
Initialize local-ci in your project (auto-detects Rust, TypeScript/Bun, or Swift):
=======
Grow Without Limits uses a **declarative GitOps model** with clear separation of concerns:

- **Flux CD**: Primary GitOps engine for cluster bootstrapping and application delivery.
- **ArgoCD**: Visual orchestrator for multi-cluster workload management.
- **Crossplane**: Control plane for managing cloud resources (AWS, GCP, Azure) as Kubernetes objects.
- **External Secrets Operator (ESO)**: Secure secret injection from AWS Secrets Manager/Azure Key Vault.
>>>>>>> main

For detailed policy, see [docs/DECLARATIVE_GITOPS_POLICY.md](docs/DECLARATIVE_GITOPS_POLICY.md).

## **Official Contact Information**

<<<<<<< HEAD
Then run:

```bash
local-ci
```

## Nix / Attic Cache

This repo includes a `flake.nix` configured with:
- `nixConfig.extra-substituters = [ "https://nix-cache.stevedores.org" ]`

Use:

```bash
nix develop
go build -o local-ci
```

## Usage

### Basic Commands

```bash
# Run default stages (fmt, clippy, test)
local-ci

# Run specific stages
local-ci fmt clippy

# Initialize project
local-ci init

# Emit machine-readable output for agents
local-ci --json

# Disable cache
local-ci --no-cache

# Auto-fix formatting
local-ci --fix

# Stop at first failure
local-ci --fail-fast

# Verbose output
local-ci --verbose

# List available stages
local-ci --list

# Print version
local-ci --version
```

### Flags

```bash
--no-cache      Disable caching, run all stages
--fix           Auto-fix issues (e.g., cargo fmt without --check)
--verbose       Show detailed output including command execution
--all           Run all stages including disabled ones
```

## Default Stages

### Rust (auto-detected via Cargo.toml)

| Stage | Command | Check | Auto-fix |
|-------|---------|-------|----------|
| **fmt** | `cargo fmt --check` | ✓ | ✓ |
| **clippy** | `cargo clippy -D warnings` | ✗ | ✗ |
| **test** | `cargo test --workspace` | ✗ | ✗ |
| **check** | `cargo check --workspace` | ✗ | ✗ |

### TypeScript/Bun (auto-detected via package.json + tsconfig/bunfig/bun.lock)

| Stage | Command | Auto-fix | Default |
|-------|---------|----------|---------|
| **typecheck** | `bun x tsc --noEmit` | ✗ | enabled |
| **lint** | `bun run lint` | ✓ (`--fix`) | enabled |
| **test** | `bun test` | ✗ | enabled |
| **format** | `bun run format --check` | ✓ | disabled |

### Swift (auto-detected via Package.swift or *.xcodeproj)

| Stage | Command (SPM) | Auto-fix | Default |
|-------|---------|----------|---------|
| **fmt** | `swift-format lint --recursive .` | ✓ | enabled |
| **lint** | `swiftlint --strict` | ✗ | disabled |
| **build** | `swift build` | ✗ | enabled |
| **test** | `swift test` | ✗ | enabled |

Xcode projects use `xcodebuild` instead of `swift` commands.

Lint and format stages delegate to your `package.json` scripts, so you can use eslint, biome, prettier, or any other tool.

## Optional Cargo Tool Stages

### cargo-deny (Security & License Checking)

Check dependencies for security vulnerabilities and license compliance:

```bash
cargo install cargo-deny

# Enable in .local-ci.toml
# [stages.deny]
# enabled = true

# Run standalone
local-ci deny
```

### cargo-audit (CVE Scanning)

Audit dependencies for known CVEs:

```bash
cargo install cargo-audit

# Enable in .local-ci.toml
# [stages.audit]
# enabled = true

local-ci audit
```

### cargo-machete (Unused Dependencies)

Find unused dependencies:

```bash
cargo install cargo-machete

# Enable in .local-ci.toml
# [stages.machete]
# enabled = true

local-ci machete
```

### taplo (TOML Formatting)

Format and lint TOML files:

```bash
cargo install taplo-cli

# Enable in .local-ci.toml
# [stages.taplo]
# enabled = true

# Format with fix
local-ci --fix taplo

# Check formatting
local-ci taplo
```

## Configuration

### .local-ci.toml

Each project can have a `.local-ci.toml` configuration file:

```toml
[cache]
# Directories to skip when computing source hash
skip_dirs = [".git", "target", ".github", "scripts", ".claude"]
# File patterns to include in hash
include_patterns = ["*.rs", "*.toml"]

[stages.fmt]
command = ["cargo", "fmt", "--all", "--", "--check"]
fix_command = ["cargo", "fmt", "--all"]
timeout = 120
enabled = true

[stages.clippy]
command = ["cargo", "clippy", "--workspace", "--all-targets", "--", "-D", "warnings"]
timeout = 600
enabled = true

[stages.test]
command = ["cargo", "test", "--workspace"]
timeout = 1200
enabled = true

# Optional tool stages...
[stages.deny]
command = ["cargo", "deny", "check"]
timeout = 300
enabled = false

[dependencies]
# System dependencies (optional)
required = []
optional = []

[workspace]
# Workspace members to exclude
exclude = []
```

### TypeScript/Bun .local-ci.toml

```toml
[cache]
skip_dirs = [".git", "node_modules", "dist", ".next", "coverage", ".claude"]
include_patterns = ["*.ts", "*.tsx", "*.js", "*.jsx", "*.json"]

[stages.typecheck]
command = ["bun", "x", "tsc", "--noEmit"]
timeout = 120
enabled = true

[stages.lint]
command = ["bun", "run", "lint"]
fix_command = ["bun", "run", "lint", "--", "--fix"]
timeout = 300
enabled = true

[stages.test]
command = ["bun", "test"]
timeout = 600
enabled = true

[stages.format]
command = ["bun", "run", "format", "--check"]
fix_command = ["bun", "run", "format"]
timeout = 120
enabled = false
```

### Default Behavior

If `.local-ci.toml` doesn't exist, local-ci auto-detects the project type and uses sensible defaults:

**Rust projects:**
- Runs: fmt, clippy, test
- Skips: .git, target, .github, scripts, .claude
- Hashes: *.rs, *.toml files

**TypeScript/Bun projects:**
- Runs: typecheck, lint, test
- Skips: .git, node_modules, dist, .next, coverage, .claude
- Hashes: *.ts, *.tsx, *.js, *.jsx, *.json files

Unknown stage names now fail fast (instead of being silently ignored).

## Caching

Cache is stored in `.local-ci-cache` (added to `.gitignore` by `local-ci init`).

**How it works:**
1. Compute MD5 hash of all Rust files in workspace
2. Skip stages if source hash matches cached hash
3. Update cache when stage succeeds

**Skip directories:**
- `.git`, `target`, `.github`, `scripts`, `.claude` (configurable)

**Force rebuild:**
```bash
local-ci --no-cache
```

## Pre-commit Hook

Initialize with optional Git pre-commit hook:

```bash
local-ci init
# Creates .git/hooks/pre-commit
```

Hook runs `local-ci fmt clippy` before allowing commits. Modify hook in `.git/hooks/pre-commit`.

Bypass hook:
```bash
git commit --no-verify
```

## Nix Cache Configuration

local-ci supports attic binary cache integration for faster Nix builds:

```bash
# Configure stevedores attic cache
local-ci configure-nix-cache
```

This adds `https://nix-cache.stevedores.org` to your Nix configuration for faster builds across all stevedores-org projects.

**Supported Caches:**
- `stevedores-attic` (https://nix-cache.stevedores.org) - Trusted cache for stevedores-org ecosystem
- `cache.nixos.org` - Official NixOS binary cache

**Manual Configuration:**

Add to `~/.config/nix/nix.conf`:
```ini
extra-substituters = https://nix-cache.stevedores.org https://cache.nixos.org
trusted-public-keys = oxidizedmlx-cache-1:uG3uzexkJno1b3b+dek7tHnHzr1p6MHxIoVTqnp/JBI= cache.nixos.org-1:6NCHdD59X431o0gWypQydGvjwydGG2UZTvhjGJNsx6E=
```

Or system-wide in `/etc/nix/nix.conf` (requires root)

## Workspace Support

local-ci automatically detects workspace structure from `Cargo.toml` (Rust) or `package.json` workspaces (TypeScript/Bun).

### Rust

```toml
[workspace]
members = ["crates/*"]
exclude = ["crates/legacy-*"]
```

**Auto-detected:**
- Workspace members
- Excluded crates (skipped in hash computation)
- Single-crate projects

### TypeScript/Bun

```json
{
  "workspaces": ["packages/*", "apps/*"]
}
```

**Auto-detected:**
- Workspace members from package.json `workspaces` globs
- Single-package projects (no workspaces field)

Configure in `.local-ci.toml`:
```toml
[workspace]
exclude = ["crates/experimental"]
```

## Output Format

```
🚀 Running local CI pipeline...

::group::fmt
$ cargo fmt --all -- --check
::endgroup::
✓ fmt (125ms)

::group::clippy
$ cargo clippy --workspace --all-targets -- -D warnings
::endgroup::
✓ clippy (2345ms)

::group::test
$ cargo test --workspace
::endgroup::
✓ test (5678ms)

📊 Summary:
  Total stages: 3
  Passed: 3
  Cached: 1 (33%)
  Executed: 2
  Total time: 8123ms

💡 Optional tools missing:
  cargo-deny:
    cargo install cargo-deny
```

## Examples

### Basic Usage

```bash
# Test before committing
local-ci

# Run single stage
local-ci fmt

# Auto-fix formatting
local-ci --fix
```

### Continuous Integration

Run in GitHub Actions:

```yaml
name: CI
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: dtolnay/rust-toolchain@stable
      - run: cargo install --locked local-ci
      - run: local-ci
```

### Development Workflow

1. Make changes to code
2. Run `local-ci` to check before pushing
3. Fix any issues (use `local-ci --fix` for formatting)
4. Commit with pre-commit hook running checks

## Troubleshooting

### "local-ci not found"

Ensure binary is in PATH:
```bash
which local-ci
export PATH="$PATH:$(go env GOPATH)/bin"  # Add to .bashrc or .zshrc
```

### Stage fails but works manually

Check if `.local-ci.toml` has correct command:
```bash
local-ci --verbose stagename
```

### Cache causing issues

Clear cache:
```bash
rm .local-ci-cache
local-ci --no-cache
```

### Missing cargo tools

Install missing tools shown in output:
```bash
cargo install cargo-deny cargo-audit cargo-machete
```

## Contributing

Bug reports and PRs welcome on GitHub: https://github.com/stevedores-org/local-ci

Each stage cache entry includes the command signature, so changing command args invalidates stale cache entries automatically.

## Agent/Automation Mode

Use `--json` for deterministic machine-readable output:

```bash
local-ci --json
```

Sample schema:
- `version`
- `duration_ms`
- `passed`
- `failed`
- `results[]` with `name`, `command`, `status`, `duration_ms`, `cache_hit`, optional `output`, optional `error`

## License

MIT

## **Official Contact Information**

- **Contact**: `contact@lornu.ai` - Official email for all external communications
- **Policy**: Only publish `contact@lornu.ai` in documentation and UI. Never publish personal or alternative email addresses.
