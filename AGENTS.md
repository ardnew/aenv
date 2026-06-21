# AGENTS.md

Guidance for AI agents working on this project.

## Commands

See [CONTRIBUTING.md](CONTRIBUTING.md) for build, test, benchmark, profile, coverage, and lint commands.

<!-- CODEGRAPH_START -->
## CodeGraph

This project has a CodeGraph MCP server (`codegraph_*` tools) configured. CodeGraph is a tree-sitter-parsed knowledge graph of every symbol, edge, and file. Reads are sub-millisecond and return structural information grep cannot.

### When to prefer codegraph over native search

- Use codegraph for **structural** questions (what calls what, what would break, where is X defined, what is X's signature, etc.)
- Use native grep/read only for **literal text** queries (string contents, comments, log messages, etc.)

| Question | Tool |
| - | - |
| "Where is X defined?" / "Find symbol named X" | `codegraph_search` |
| "What calls function Y?" | `codegraph_callers` |
| "What does Y call?" | `codegraph_callees` |
| "How does X reach/become Y? / trace the flow from X to Y" | `codegraph_trace` (one call = the whole path, incl. callback/React/JSX dynamic hops) |
| "What would break if I changed Z?" | `codegraph_impact` |
| "Show me Y's signature / source / docstring" | `codegraph_node` |
| "Give me focused context for a task/area" | `codegraph_context` |
| "See several related symbols' source at once" | `codegraph_explore` |
| "What files exist under path/" | `codegraph_files` |
| "Is the index healthy?" | `codegraph_status` |

### Rules of thumb

- **Answer directly — don't delegate exploration.**
  - For **structural** ("***what*** is *X*", "***how*** is *X* defined", etc.), answer with 2 codegraph calls:
    1. `codegraph_context`
    2. `codegraph_explore` (**ONCE**) for the source of the symbols it surfaces.
  - For **control-flow** ("***how*** does *X* reach *Y*", "***when*** does *X* affect *Y*"), answer with 2 codegraph calls:
    1. `codegraph_trace` *X*→*Y*
       - One call returns the whole path with dynamic hops bridged.
    2. `codegraph_explore` (**ONCE**) for the body of the source.
  - **NEVER** rebuild the index with `codegraph_search` + `codegraph_callers`.
    - Codegraph **IS** the pre-built index.
- **Trust codegraph results.**
  - They come from a full AST parse.
  - **NEVER** re-verify them with grep.
- **NEVER chain `codegraph_search` + `codegraph_node`** when you just want context.
  - Use `codegraph_context` (**ONCE**).
- **NEVER loop `codegraph_node` over many symbols**
  - Use `codegraph_explore` (**ONCE**)
- **Index lag — check the staleness banner, don't guess a wait.**
  - When a codegraph response starts with "*⚠️ Some files referenced below were edited since the last index sync…*":
    - The listed files are pending re-index.
    - Read those specific files for accurate content.
    - All files **NOT** mentioned in the response are fresh and codegraph is authoritative for them.
  - `codegraph_status` also lists pending files under "Pending sync".

### If `.codegraph/` doesn't exist

Get punched.
<!-- CODEGRAPH_END -->
