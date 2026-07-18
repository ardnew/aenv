# aenv

`aenv` generates process environments from isolated, prefix-based package installations.
Its configuration language models reusable, parameterized layers whose ordered composition determines values such as
`PATH`, `LD_LIBRARY_PATH`, `MANPATH`, and `PKG_CONFIG_PATH` without modifying the package installations.

The interactive REPL exists to evaluate, inspect, and explain those environments.
It is an interface to the environment engine, not the purpose of the engine itself.

See [DESIGN.md](DESIGN.md) for the project's foundational design philosophy, scope, non-goals, and boundaries.
