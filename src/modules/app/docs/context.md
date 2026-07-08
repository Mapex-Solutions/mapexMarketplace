# Bounded Context: App (module bootstrap)

Last reviewed: 2026-07-04

## Purpose

Coordinate startup of every other module. `app` is **not a domain context** — it
owns no catalog, entity, or business rule. Its single responsibility is running
each registered module's init phases in a fixed order so the dependency-injection
container is fully wired before the HTTP server accepts traffic. It is the one
place that knows "initialize all modules," so no individual module has to know
about the others.

## Ubiquitous Language

- **Module** — a self-contained bounded context registered in the shared module
  registry (`src/shared/configuration/modules`), exposing optional
  `InitRepositories` / `InitServices` / `InitInterfaces` phase hooks.
- **Init phase** — one of the three ordered startup stages. All modules complete
  a phase before any module starts the next, so services can resolve repositories
  and interfaces can resolve services.

## Published Events

- None. This context emits no messages.

## Consumed Events

- None.

## Driving Ports

- None. `InitModule()` is invoked directly by the service bootstrap, not through a
  port interface.

## Driven Ports

- None. It calls the phase hooks each module registered in the shared module
  registry; it holds no repository or client of its own.

## Invariants

- Phases run strictly in order across all modules — every module's repositories
  are initialized before any module's services, and all services before any
  interfaces — so a later phase can always resolve what an earlier phase provided.
- The loop is wiring-agnostic: it never references a concrete module, so adding a
  module changes only the registry, never this code.

## Cross-Context Interactions

- Invoked once by the service bootstrap during startup; drives the init hooks of
  the `assettemplates`, `devices`, and `workflowplugins` modules.
