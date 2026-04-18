# Poker Project — Convenções

## Stack

| Camada | Tecnologia |
|--------|-----------|
| Backend | Go 1.23+, Gin, pgx/v5, zerolog, golang-jwt/v5, nhooyr.io/websocket |
| Frontend | React 18, Vite, TypeScript (strict), Tailwind CSS v3 |
| Estado cliente | Zustand (auth/UI global), TanStack Query (dados servidor) |
| DB | PostgreSQL via pgx/v5 (sem ORM) |
| Cache/PubSub | Redis via go-redis/v9 |
| Migrations | golang-migrate (arquivos em `poker-backend/migrations/`) |
| WebSocket | nhooyr.io/websocket no backend, classe PokerSocket no frontend |

## Estrutura de pastas

```
poker/
├── poker-backend/
│   ├── cmd/server/main.go          # entrypoint
│   ├── internal/
│   │   ├── auth/                   # JWT, middleware, handlers de login/register
│   │   ├── user/                   # entidade User, repositório, handlers
│   │   ├── table/                  # Mesa de poker, lobby
│   │   ├── game/                   # Engine do jogo (mãos, ações, pot)
│   │   ├── ws/                     # Hub WebSocket, mensagens, broadcast
│   │   ├── db/                     # Conexão pgx pool
│   │   ├── cache/                  # Conexão Redis
│   │   └── config/                 # Config via env vars
│   ├── migrations/                 # .up.sql / .down.sql numerados
│   └── pkg/                        # Helpers reutilizáveis sem dependência de internal
└── poker-frontend/
    └── src/
        ├── api/           # axios client + funções de chamada REST
        ├── ws/            # PokerSocket (WebSocket singleton)
        ├── stores/        # Zustand stores (auth, game, ui)
        ├── features/      # Uma pasta por feature (auth/, lobby/, table/, game/)
        ├── components/    # Componentes UI genéricos (Button, Card, Modal…)
        ├── hooks/         # Custom hooks reutilizáveis
        ├── types/         # Interfaces TypeScript compartilhadas
        └── routes/        # Definição de rotas React Router
```

## Convenções de código

### Backend (Go)
- Sem ORM: queries SQL diretas com pgx
- Cada pacote em `internal/` expõe apenas interfaces; structs concretas são privadas
- Handlers recebem `*gin.Context`; lógica de negócio fica em serviços separados
- Erros sempre propagados com `fmt.Errorf("contexto: %w", err)`
- Logging com `zerolog` — nunca `fmt.Println` em produção
- Variáveis de ambiente lidas apenas em `internal/config`
- Migrations nomeadas: `000001_create_users.up.sql` / `.down.sql`

### Frontend (TypeScript/React)
- TypeScript strict — sem `any`, sem `// @ts-ignore`
- Componentes: arrow functions exportadas como `export function`
- Estado servidor → TanStack Query; estado global client-side → Zustand
- Estilos: só Tailwind utility classes (sem CSS modules, sem styled-components)
- Imports com alias `@/` mapeando para `src/`
- Sem `useEffect` para fetch — usar `useQuery`/`useMutation`
- Tipos compartilhados definidos em `src/types/index.ts`

### WebSocket — protocolo de mensagens
```json
{ "type": "ACTION_NAME", "payload": { ... } }
```
Tipos definidos como constantes em `poker-backend/internal/ws/messages.go` e espelhados em `poker-frontend/src/ws/types.ts`.

## Fluxo de autenticação
1. `POST /api/auth/register` → cria usuário, retorna JWT
2. `POST /api/auth/login` → valida credenciais, retorna JWT
3. Token armazenado em `localStorage` via `useAuthStore`
4. Axios interceptor injeta `Authorization: Bearer <token>` em toda request
5. Middleware Gin valida JWT antes de rotas protegidas

## Comandos úteis

```bash
# Backend — dev com hot reload (requer air)
cd poker-backend && air

# Backend — build manual
cd poker-backend && go build -o ./tmp/server ./cmd/server

# Backend — rodar migrations
migrate -path ./migrations -database $DATABASE_URL up

# Frontend — dev
cd poker-frontend && npm run dev

# Frontend — typecheck
cd poker-frontend && npm run typecheck
```

## Regras gerais
- Sem comentários óbvios; só comentar WHY, nunca WHAT
- Sem features extras além do que foi pedido na tarefa
- PRs pequenos e focados em uma única mudança
- Cada migration deve ter o `.down.sql` correspondente
