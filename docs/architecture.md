# Architecture

```mermaid
flowchart LR
    U[Browser / curl] -- "HTTPS :8443" --> P

    subgraph host[Docker host]
      subgraph frontend[network: frontend]
        P[Caddy proxy<br/>TLS internal CA<br/>security headers]
      end
      subgraph backend[network: backend - internal, no internet]
        A[Go API :8080<br/>non-root, read-only rootfs<br/>cap_drop ALL]
      end
      V[(volume school-data<br/>/data/school.db)]
      B[(./backups<br/>verified snapshots + sha256)]
    end

    P -- "HTTP" --> A
    A --- V
    V -. "scripts/backup.sh<br/>VACUUM INTO" .-> B
    B -. "scripts/restore.sh" .-> V

    subgraph ci[GitHub Actions]
      C1[gofmt / vet / staticcheck / tests / govulncheck] --> C2[build + Trivy scan] --> R[(GHCR image<br/>sha / semver / latest)]
    end
    R -. "docker pull, deploy.sh vX.Y.Z" .-> A
```

## Database schema

```mermaid
erDiagram
    users ||--o{ teacher_assignments : "teacher_id"
    classes ||--o{ teacher_assignments : "class_id"
    classes ||--o{ students : "class_id"
    students ||--o{ attendance : "student_id"
    users ||--o{ attendance : "recorded_by"
    users ||--o{ audit_logs : "actor_id"
    users { int id PK  text name  text role  text token_hash }
    classes { int id PK  text name  int grade  text section }
    students { int id PK  text name  int class_id FK }
    teacher_assignments { int teacher_id PK  int class_id PK }
    attendance { int id PK  int student_id  int class_id  text attendance_date  text session  text status  int recorded_by }
    audit_logs { int id PK  int actor_id  text action  int entity_id  text detail  text created_at }
```

`attendance` has `UNIQUE(student_id, class_id, attendance_date, session)` and `CHECK` constraints on
`status` (`PRESENT`, `ABSENT`) and `session` (`MORNING`, `AFTERNOON`).

## Security design

- Only the proxy is published; the API sits on an `internal` network with no route to the internet.
- API container: non-root (uid 65532), distroless image, read-only root filesystem, all capabilities dropped, `no-new-privileges`.
- Proxy: all capabilities dropped except `NET_BIND_SERVICE`; HSTS and other security headers; Caddy admin API disabled.
- Tokens are hashed (SHA-256) at rest; authorization (role + class assignment) is enforced server side, and the actor id always comes from the authenticated token, never the request body.
- Configuration is via environment variables; no secrets are committed (`.env` is ignored).
- CI uses least-privilege permissions (`contents: read`; `packages: write` only in the publish job).
