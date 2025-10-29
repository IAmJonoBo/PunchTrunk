# ADR-0001: Record Architecture Decisions

_Status:_ Accepted  
_Date:_ 2025-10-29

## Context

We need a durable, searchable log of significant technical decisions.

## Decision

Use **Architecture Decision Records (ADRs)** in the Nygard format (context → decision → consequences). One ADR per decision, immutable once accepted and stored in `docs/architecture/ADRs` with incremental numbering.

## Consequences

- We gain traceability and rationale over time.
- Lightweight overhead; encourages deliberate choices.
- See: <https://adr.github.io> and <https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions>
