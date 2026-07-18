# Design Philosophy

## Purpose

`aenv` constructs process environments from standalone package installations
arranged in Unix-style prefix trees. A package installed beneath
`/opt/pkg`, for example, may contribute `/opt/pkg/bin` to `PATH`,
`/opt/pkg/lib` to a loader search path, or other package-specific values.

The project exists because the relationship between an installation prefix and
its useful environment cannot be hardcoded correctly for every package and
platform. Instead, users describe that relationship as reusable,
parameterized, immutable layers. Ordering layers is an explicit policy choice:
when multiple packages provide the same command or resource, composition order
can determine which one is found first.

In this project, an **environment** means the environment-variable mapping
passed to a process. It does not mean a container, chroot, filesystem image, or
complete emulation of a host.

## Core model

A layer is a declarative contribution to a process environment. It may be
parameterized by facts such as an installation prefix or platform, instantiated
without mutating its definition, and combined with other layers in an explicit
order.

The core engine must support four operations:

1. Define and reuse parameterized layers.
2. Compose instantiated layers in a user-controlled order.
3. Materialize the resulting process environment for inspection or execution.
4. Explain where each resulting value or path entry came from.

Unix filesystem conventions are inputs to user-authored policy, not universal
truths embedded in the evaluator. The project may eventually ship convenience
layers for common conventions, but the language and engine must also represent
bespoke layouts without adding package-specific behavior to the evaluator.

The REPL is an interactive client of this engine. Its job is to evaluate
expressions, display results, expose provenance, and make failures
understandable. Evaluation must remain separable from terminal UI so the same
engine can later support command execution and non-interactive use without
duplicating semantics.

## Design principles

- **Read-only construction.** Evaluating an environment does not create,
  remove, or rearrange package files. It avoids symlink farms and respects the
  filesystem permissions already in place.
- **Explicit policy.** Prefix layout, variable selection, ordering, inclusion,
  and exclusion are configuration decisions. Hidden package detection must not
  silently choose them for the user.
- **Order is observable.** Precedence-sensitive values retain a predictable
  relationship to layer order. The evaluator must not reorder contributions for
  convenience.
- **Immutable composition.** Instantiating or composing a layer produces a new
  value and does not alter the layer or another previously computed result.
- **Explainability.** A final environment alone is insufficient. Users must be
  able to inspect the layer and input responsible for a value, especially when
  diagnosing precedence.
- **Determinism with explicit inputs.** The same definitions, parameters,
  baseline environment, platform facts, and filesystem observations must
  produce the same result. Any dependency on host state must be visible rather
  than ambient and accidental.
- **Mechanism over catalog.** The core provides a small mechanism for describing
  environments. An ever-growing catalog of conventions and package recipes is
  separate policy and must not expand the language core.
- **Errors over guesses.** Ambiguous or invalid composition should produce a
  useful diagnostic instead of a plausible but unexplained environment.

## Required practical outcomes

The first complete implementation is successful when it can demonstrate all of
the following without package-specific evaluator code:

- Model several isolated prefixes, such as `/opt/pkg`, `/opt/app`, and
  `/opt/proj`, using reusable definitions.
- Compose their contributions so changing layer order predictably changes
  command and resource precedence.
- Represent common path-list variables, ordinary scalar variables, and explicit
  removal or omission where required by a package configuration.
- Inspect both the materialized environment and the provenance of its parts.
- Launch an executable with the materialized environment while leaving the
  installation trees untouched.
- Report invalid definitions, missing required inputs, and unsupported
  operations with source-oriented diagnostics.
- Exercise the same evaluation behavior from the REPL and from tests or a
  non-interactive caller.

These outcomes are the acceptance boundary. Support for another convention,
package, platform, or interface is not a prerequisite unless one of these
outcomes cannot be met without it.

## Non-goals

`aenv` is not:

- a package installer, uninstaller, upgrader, or package database;
- a dependency solver or a system for fetching/building packages;
- a symlink-farm or filesystem-overlay manager;
- a sandbox, container runtime, permission boundary, or security mechanism;
- a general-purpose shell or programming language;
- an automatic detector of every usable directory beneath a prefix;
- a mechanism for repairing binaries whose paths or runtime dependencies were
  fixed at build time; or
- a guarantee that a platform's loader, security policy, or executable will
  honor a particular environment variable.

The read-only guarantee applies to environment construction by `aenv`. A
program launched with the resulting environment may still write files, start
services, or have other side effects. Environment composition also provides no
isolation: conflicting files remain on disk, secrets placed in the environment
remain visible under the host's normal rules, and platform restrictions such as
secure-execution modes may ignore loader variables.

Configuration directories illustrate another boundary. A path such as
`${PREFIX}/etc` has no single portable environment variable that makes every
application use it. A layer can express a variable understood by a particular
application, but `aenv` must not pretend that filesystem convention alone
changes application behavior.

## Scope control

A proposed feature belongs in the core only when all of these are true:

1. It is necessary to construct, combine, inspect, or apply a process
   environment.
2. It is demonstrated by a concrete package or platform scenario rather than a
   hypothetical desire for completeness.
3. Existing layer composition cannot express it without violating a core
   principle.
4. Its semantics can be specified deterministically and tested independently of
   the REPL UI.
5. It does not turn the evaluator into a package manager, shell, filesystem
   mutator, or unbounded catalog of conventions.

Features that fail this test should live in user configuration, a reusable
policy library, an adapter, or a separate tool. They should not be added to the
language merely because another environment-related system supports them.

## Implementation order

Language syntax should be designed only after the behavioral contract is
settled. Work should proceed in this order:

1. Turn the required practical outcomes into end-to-end examples and acceptance
   tests.
2. Specify the environment value model, composition rules, inputs, provenance,
   and error behavior independently of syntax.
3. Implement and test the evaluator as a UI-independent component.
4. Design the smallest language capable of expressing the accepted scenarios.
5. Add process application and command execution around the evaluator.
6. Build the REPL as an inspection and experimentation interface over the same
   APIs.

Later phases may add conveniences, optimizations, or policy libraries, but they
must preserve this boundary and be justified by real cases.
