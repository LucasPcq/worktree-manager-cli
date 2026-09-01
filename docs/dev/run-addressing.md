# Named URLs — the vocabulary

Three different things were being called "the proxy": the server that makes names
resolve, the OS rule that removes a port from those names, and the port either of them
happens to use. They are separate mechanisms with separate failure modes, and confusing
them makes every message about them unreadable. This page fixes the words. Code,
constants and CLI copy use these and no synonyms.

## The seven terms

| Term | What it is | Where it lives |
| -- | -- | -- |
| **the proxy** | The reverse proxy the daemon runs. It listens on one port and routes by `Host` header to the job answering under that name. **This is what makes names exist at all.** On by default; `[proxy] enabled = false` in the global config switches it off | `internal/service/proxy` |
| **bind port** | What the proxy actually listens on: `10080`, or the next free port when that one is taken (the R7 fallback) | `rules.ProxyPort`, then the daemon's own fallback |
| **privileged redirection** | The OS rule `wtm run proxy install` installs — on macOS a launchd agent binding `:80` and relaying to the bind port. **It only removes the port suffix from a URL.** It unlocks nothing | `internal/service/proxy/redirect*.go` |
| **public port** | What a URL *announces*, as opposed to what anything binds: nothing (i.e. `:80`) when the redirection is live, the bind port otherwise | `rules.PublicPort` |
| **route host** | `<job>.<worktree>.<project>.localhost` — the name a published job answers under | `rules.RouteHost` |
| **origin** | `scheme://host[:port]` — the thing a browser sends in `Origin:` and CORS compares. The port is part of an origin; it is **not** part of a cookie's origin | `rules.JobOrigin` |
| **addressing** | `ports` or `names`: which of the two an `[[env_port]]` link writes into a `.env` value | `rules.EffectiveAddressing` |

## What follows from the distinction

**The proxy makes names work. The redirection makes them pretty.** `*.localhost`
resolves to `127.0.0.1` natively (RFC 6761), but resolving is not answering — without
the proxy listening, a name gives `connection refused`. Without the redirection, the
same name works perfectly, carrying `:10080`.

| State | What a `.env` holds | Works? |
| -- | -- | -- |
| proxy on, redirection **not** installed | `http://api-dev.feat-x.monorepo.localhost:10080` | yes |
| proxy on, redirection installed | `http://api-dev.feat-x.monorepo.localhost` | yes |
| proxy off (`enabled = false`) | `http://localhost:4011` | yes, without names |

**A port in the URL costs nothing that matters.** CORS compares whole origin strings,
so as long as both sides carry the same one — port included — it passes. And cookie
isolation, the reason named URLs exist, keys on the **host** and ignores the port
entirely: `feat-x` and `feat-y` are separate jars whether or not `:10080` is there.

**Addressing is a project setting, the proxy is a machine setting.** `run.toml` says
what the project wants; the global config says what this machine can do. When
the machine cannot honour it — proxy off, no public port — ports are written and a
notice says so. Writing a name nothing serves would produce a syntactically perfect,
dead value.

## Addressing in `run.toml`

```toml
addressing = "names"   # the default — omit it
```

Under `names`, an `[[env_port]]` link writes the job's full origin instead of its port
number, but **only when two conditions hold at once**:

1. the job it names **publishes a url**, and that url publishes the very port the link
   follows;
2. the value in the `.env` **has the shape of a URL**.

Both are load-bearing. The first excludes Postgres — `DATABASE_URL → docker-compose.POSTGRES_PORT`
has no name and never will, since the proxy only speaks HTTP. The second excludes the
binding keys: `apps/api/.env: PORT → api-dev.PORT` names a published job and must stay
a bare number. Neither condition alone is enough.

```
apps/api/.env   PORT=4001         → 4011                                         (bare number)
apps/web/.env   VITE_API_URL      → http://api-dev.feat-x.monorepo.localhost:10080
                CORS_ORIGIN       → http://web-dev.feat-x.monorepo.localhost:10080
                DATABASE_URL      → …@localhost:5442/db                           (no name)
```

`addressing = "ports"` is the escape hatch, and it is a real inverse: a value wtm wrote
as an origin is recognised as such and rendered back to `http://localhost:<base+offset>`.

## Recognising wtm's own writing

A value already carrying a route host is recognised **structurally** — the authority
matches `<job label>.<anything>.<project label>.localhost` — rather than remembered in a
state file. One primitive then answers four questions that would otherwise each need
their own mechanism:

- a second `wtm env` recomputes the same origin, so nothing is rewritten twice;
- a `.env` copied from main or a parent carries that worktree's segment, which is
  corrected to this one;
- installing the redirection after the fact changes the public port, and the next pass
  drops the `:10080`;
- switching to `addressing = "ports"` knows which values were wtm's to undo.

A state file would be more precise on paper and worse in practice: it diverges the
moment somebody edits a `.env` by hand.

## Dev servers and the `Host` header

A named URL reaches the job with a `Host` the dev server has never seen, and some
servers refuse one they do not recognise. Two of them matter here, and they do not
need the same thing:

- **Next** refuses it unless the host is listed in `allowedDevOrigins`. `wtm run up`
  reports the missing entry when it serves a job whose directory holds a
  `next.config.*` without one (`rules.NeedsDevOrigins`, `rules.DevOriginsPattern`).
- **Vite** needs nothing. Its host check allows `localhost` and **every** `.localhost`
  subdomain outright, before `server.allowedHosts` is even consulted — verified in
  Vite 8.2.2, `isHostAllowedInternal`. There is nothing for wtm to detect or report.

Anything else is a report from a user, not a guess from us: the presence of a config
file is the signal, never a framework inferred from a command line.

## The trade the mode makes

Under `names`, **the named URL becomes the only working entrance.** Opening
`localhost:5183` directly sends an `Origin` the API no longer knows, and CORS blocks it.
`wtm run url` and `wtm run open` hand out the right link; a bookmark on the raw port
stops working. That is the cost of having two worktrees stay logged in at the same time.

## Related

- `internal/rules/envorigins.go` — the whole origin surgery, pure and testable
- `internal/rules/envports.go` — the port substitution it sits beside
- [architecture.md](architecture.md) — which layer may call which
