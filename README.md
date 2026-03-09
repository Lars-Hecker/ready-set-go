# Ready Set Go 🚀
**A ready-to-use Go backend boilerplate.**

This repository provides a preconfigured, minimal application foundation. The tech stack is chosen for speed, simplicity, and robustness, strictly favoring explicit clarity over hidden magic. 

While batteries-included frameworks can speed up development speed, their lack of flexibility can significantly impact scalability, especially over time as architectural practices evolve. Therefore, this codebase curates established, high-performance tools into a cohesive starting app that does not rely on any web framework. Bypass the friction of initial setup and focus directly on building core features.

## Features & Stack

**Core Features**
- Users, Workspaces, and Memberships
- Slack-inspired policy-based RBAC
- Passwordless Auth with support for 2FA
- TODO: High Test Coverage
- Several Utilities (slog logging, redis queues, ...)

**Addons**
- Admin Panel with Content and User Management
- Notifications/Email (using AWS)
- Integration support via OAuth2
- Billing Setup (using Stripe)
- S3 File Upload (using Presigned URLs)

**Tech Stack**
- **Shared:** Proto/RPC
- **Backend:** Go with PGX, SQLC, ConnectRPC, Casbin, OAuth2
- **Frontend:** Vite + React with Zustand, ReactQuery

## Getting Started 🛠️
*Note: This project is currently a Work in Progress.*

### Quickstart Guide
1. Update the `.env`, similar to `.env.example`.
2. Run `docker up`.

### Architecture & Workflow
This application relies on a **schema-first approach** to keep the frontend and backend in sync:
1. **Define the contract:** Start by defining your API layer in the `proto/` directory. 
2. **Generate code:** Generate the necessary boilerplate for both the frontend and backend (found in their respective `gen/` folders).
3. **Implement:** Develop the frontend and backend in parallel based on the shared contract.

### Data Layer
The Go backend prioritizes execution speed and simplicity. We use **PostgreSQL** for its reliable performance and flexibility, utilizing it for standard relational data as well as indexed, unstructured JSONB. 

To keep database interactions transparent and scalable, we avoid ORMs. Instead, we write pure SQL (located in `sql/`) and pair it with **SQLC** for type-safe Go code generation.

### API
TODO: why ConnectRPC, How connectrpc
Generated code for the api is found in `api/gen`. The Handlers are written using ConnectRPC. To understand the corresponding implementations in `api/domain`, we suggest checking out their documentation.

### Frontend
To provide a consistent experience between the react apps, we provide libraries in `package/`. Currently implemented are a shadcn-based ui library and the domain logic.
TODO 

### Admin Panel
TODO: how to wire up content management pages and user management

### Key Directories to Explore
- **`api/infra/pref/`**: Understand how roles and permissions (RBAC) are structured.
- **`api/infra/auth/`**: Review the session-based authentication flow.
