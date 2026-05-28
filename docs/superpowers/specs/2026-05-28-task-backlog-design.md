# Task Backlog Design

## Context

The workspace started empty.
The goal is to create a very small Go project that teaches strict clean architecture and hexagonal architecture while also making the data flow easy to follow.

The user also asked for:

- heavy explanatory comments
- structs and interfaces used clearly
- a file-based SQLite database
- a stricter outside-to-inside dependency flow

## Candidate approaches

### Option 1: HTTP + SQLite + single aggregate

Use a small HTTP API, one aggregate root (`Task`), one application service, one repository port, and a SQLite adapter.

Trade-offs:

- best balance between realism and size
- clearly shows an inbound adapter and an outbound adapter
- easy to trace request flow end to end

### Option 2: CLI + SQLite

Use a command-line adapter instead of HTTP.

Trade-offs:

- even smaller
- easier to run manually
- less representative of a typical service boundary

### Option 3: HTTP + multiple aggregates

Use a richer domain like projects plus tasks.

Trade-offs:

- teaches more DDD vocabulary
- adds complexity quickly
- makes the architecture harder to read for a first example

## Chosen approach

Option 1 was selected autonomously because the session is in autonomous mode and the user is not available for follow-up questions.

## Architecture

### Domain layer

The domain layer owns business rules.
It defines the `Task` aggregate root and the allowed lifecycle transitions.
It has no knowledge of HTTP, SQLite, JSON, or framework code.

### Port layer

Inbound ports define the use cases the outside world is allowed to call.
Outbound ports define the capabilities the application layer needs from the outside world.

### Application layer

The application layer implements the use cases.
It orchestrates the domain model and calls outbound ports.
It does not know which adapter fulfills those ports.

### Adapter layer

Inbound adapters translate transport details into use-case calls.
Outbound adapters translate port calls into infrastructure behavior.

### Composition root

`main.go` is the outermost layer.
It creates concrete adapters and injects them into the application service.

## Planned request flow

1. An HTTP client sends a request.
2. The HTTP adapter validates and translates the request.
3. The adapter calls an inbound port.
4. The application service executes a use case.
5. The application service uses the domain model.
6. The application service calls an outbound port.
7. The SQLite adapter persists or reads state.
8. Control returns back outward as a response DTO.

## Error handling

- domain rule violations become application errors returned to adapters
- not-found repository results become port-level errors
- the HTTP adapter maps those errors to HTTP status codes

## Testing plan

- domain tests for task lifecycle rules
- application tests with fake outbound adapters
- SQLite adapter round-trip test against a temporary file database

## Notes

The brainstorming skill normally asks for explicit user approval before implementation.
That approval step could not happen because the environment reported that the user is unavailable and asked the agent to proceed autonomously.
