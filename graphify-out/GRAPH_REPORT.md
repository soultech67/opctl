# Graph Report - cli  (2026-04-12)

## Corpus Check
- Corpus is ~19,079 words - fits in a single context window. You may not need a graph.

## Summary
- 429 nodes · 536 edges · 49 communities detected
- Extraction: 93% EXTRACTED · 7% INFERRED · 0% AMBIGUOUS · INFERRED: 38 edges (avg confidence: 0.82)
- Token cost: 0 input · 0 output

## God Nodes (most connected - your core abstractions)
1. `FakeCLIParamSatisfier` - 38 edges
2. `FakeCliColorer` - 36 edges
3. `CliOutput` - 15 edges
4. `CLI Integration Smoke Suite` - 13 edges
5. `CLI Param Satisfier` - 10 edges
6. `FakeInputSrc` - 9 edges
7. `FakeInputSourcer` - 9 edges
8. `Command Registry` - 9 edges
9. `Op Execution Flow` - 8 edges
10. `CLI Output Implementation` - 8 edges

## Surprising Connections (you probably didn't know these)
- `CLI for opctl` --conceptually_related_to--> `Root Command`  [INFERRED]
  cli/README.md → cli/cmd/root.go
- `Cobra Command Architecture` --conceptually_related_to--> `Root Command`  [INFERRED]
  cli/CONTRIBUTING.md → cli/cmd/root.go
- `Go SDK Reuse` --conceptually_related_to--> `Op Execution Flow`  [INFERRED]
  cli/CONTRIBUTING.md → cli/cmd/run.go
- `Embedded Web UI` --conceptually_related_to--> `Web UI Mount Flow`  [INFERRED]
  cli/CONTRIBUTING.md → cli/cmd/ui.go
- `Interface-First Testing` --conceptually_related_to--> `CLI Integration Smoke Suite`  [INFERRED]
  cli/CONTRIBUTING.md → cli/cli_test.go

## Hyperedges (group relationships)
- **Primary Command Surface** — events_event_streaming, ls_operation_listing, run_op_execution_flow, selfupdate_self_update_flow, ui_web_ui_mount_flow [EXTRACTED 1.00]
- **Commands That Auto-Start a Node** — auth_add_registry_auth, events_event_streaming, ls_operation_listing, run_op_execution_flow, ui_web_ui_mount_flow [INFERRED 0.92]
- **Node Lifecycle Management** — node_container_runtime_selection, node_node_bootstrap_flow, node_destructive_delete, node_non_destructive_kill, selfupdate_self_update_flow [INFERRED 0.84]
- **Terminal Interaction Stack** — clicolorer_colorer_implementation, clioutput_output_implementation, cliparamsatisfier_param_satisfier_implementation [INFERRED 0.85]
- **Input Resolution Pipeline** — cliparamsatisfier_param_satisfier_implementation, inputsourcer_input_source_chain, inputsrcfactory_input_source_factory, cliparamsatisfier_typed_param_coercion [INFERRED 0.90]
- **Event Rendering Pipeline** — clioutput_output_implementation, clioutput_event_renderer, formatopref_op_reference_formatter, clicolorer_terminal_color_roles [INFERRED 0.88]
- **InputSrc Implementers** — cliprompt_cli_prompt_input_src, envvar_env_var_input_src, paramdefault_param_default_input_src, slice_slice_input_src, ymlfile_yml_file_input_src, fakes_fake_input_src [EXTRACTED 1.00]
- **Single-Read Sourcing Pattern** — envvar_env_var_input_src, paramdefault_param_default_input_src, slice_slice_input_src, ymlfile_yml_file_input_src [INFERRED 0.88]
- **Generated Counterfeiter Doubles** — fakes_fake_input_src, fakes_fake_input_sourcer, fakes_fake_cli_param_satisfier [EXTRACTED 1.00]
- **Interactive Auth Recovery Flow** — dataResolver_data_resolver, dataResolver_auth_retry, dataResolver_credential_prompt_inputs, dataResolver_github_pat_guidance [EXTRACTED 1.00]
- **Local Node Lifecycle Management** — nodeProvider_node_provider_interface, local_local_node_provider, local_api_client_probe, local_daemonized_node_launch, local_pidfile_shutdown, ensure_sudo_reexec_guard [EXTRACTED 1.00]
- **Terminal Execution Visualization** — callgraph_execution_call_graph, callgraph_collapsed_completed_branches, callgraph_conditional_skip_rendering, callgraph_warning_stream, loading_loading_spinner [EXTRACTED 1.00]
- **Resettable Terminal Output Flow** — outputmanager_output_manager, outputmanager_terminal_width_provider, outputmanager_terminal_clearing, outputmanager_width_limited_rendering [INFERRED 0.87]
- **PID Lock Coordination Flow** — constructpidfilepath_pid_lock_path_construction, pidfile_pid_lock_file, getpidfromfile_pid_file_reader, trygetprocess_pid_backed_process_probe, trygetlock_pid_lock_acquisition [INFERRED 0.91]

## Communities

### Community 0 - "Command Wiring Layer"
Cohesion: 0.04
Nodes (12): CliColorer, CLIParamSatisfier, getSortedParamNames(), DataResolver, New(), NodeConfig, nodeProvider, NodeProvider (+4 more)

### Community 1 - "Fake Satisfier Harness"
Cohesion: 0.06
Nodes (1): FakeCLIParamSatisfier

### Community 2 - "Fake Colorer Harness"
Cohesion: 0.07
Nodes (1): FakeCliColorer

### Community 3 - "Validation and Terminal UX"
Cohesion: 0.09
Nodes (33): cliColorer Implementation, CliColorer Interface, Disable Color Mode, Fake CliColorer, Terminal Color Roles, Event Renderer, CLI Output Implementation, CliOutput Interface (+25 more)

### Community 4 - "CLI Command Surface"
Cohesion: 0.11
Nodes (32): Registry Auth Storage, Auth Command Group, Cobra Command Architecture, Embedded Web UI, Depend on interfaces and fakes, Interface-First Testing, Keep non-CLI behavior in the SDK, Go SDK Reuse (+24 more)

### Community 5 - "Input Source Factory"
Cohesion: 0.08
Nodes (7): InputSrcFactory, cliPromptInputSrc, envVarInputSrc, InputSrc, paramDefaultInputSrc, sliceInputSrc, ymlFileInputSrc

### Community 6 - "Parameter Input Sources"
Cohesion: 0.15
Nodes (18): CLI Prompt Input Source, Secret Input Masking, Terminal-Only Prompting, Environment Variable Input Source, CLIParamSatisfier Interface, CLIParamSatisfier Fake, InputSourcer Fake, InputSrc Fake (+10 more)

### Community 7 - "Data Resolution and Nodes"
Cohesion: 0.15
Nodes (17): Interactive Auth Retry, Credential Prompt Inputs, Data Resolver, Local Filesystem Resolution, GitHub PAT Guidance, Node-backed Resolution, Auth Prompt Contract, Filesystem and Node Fallback Contract (+9 more)

### Community 8 - "CliOutput Methods"
Cohesion: 0.28
Nodes (1): CliOutput

### Community 9 - "Input Sourcer Double"
Cohesion: 0.17
Nodes (2): InputSourcer, FakeInputSourcer

### Community 10 - "Call Graph Core"
Cohesion: 0.33
Nodes (3): newCallGraphNode(), CallGraph, callGraphNode

### Community 11 - "Execution Graph Rendering"
Cohesion: 0.22
Nodes (11): Collapsed Completed Branches, Conditional Skip Rendering, Event-driven Call Updates, Execution Call Graph, Call Graph Rendering Contract, Warning Stream, ANSI-safe Truncation, Visible Width Formatting Contract (+3 more)

### Community 12 - "Output Manager"
Cohesion: 0.33
Nodes (11): ANSI-Aware Width Measurement, NewOutputManager, OutputManager, Terminal Clearing, Terminal Width Provider, Clear Behavior Test, Long Line Truncation Test, NewOutputManager Test (+3 more)

### Community 13 - "InputSrc Fake"
Cohesion: 0.25
Nodes (1): FakeInputSrc

### Community 14 - "PID Locking"
Cohesion: 0.43
Nodes (8): PID Lock Path Construction, PID File Reader, pid.lock File, PID Lock Acquisition, Overwrite Stale pid.lock After Dead Process Check, Treat Non-Running Process as Released Lock, Treat Missing pid.lock as Already Stopped, PID-backed Process Probe

### Community 15 - "Loading Spinners"
Cohesion: 0.4
Nodes (3): DotLoadingSpinner, LoadingSpinner, StaticLoadingSpinner

### Community 16 - "ANSI Formatting Tests"
Cohesion: 0.33
Nodes (0):

### Community 17 - "Fake Writer"
Cohesion: 0.33
Nodes (1): fakeWriter

### Community 18 - "Output Manager Tests"
Cohesion: 0.4
Nodes (0):

### Community 19 - "Output Manager API"
Cohesion: 0.4
Nodes (1): OutputManager

### Community 20 - "ANSI Formatting Helpers"
Cohesion: 0.83
Nodes (3): countChars(), stripAnsi(), stripAnsiToLength()

### Community 21 - "Call Graph Tests"
Cohesion: 0.5
Nodes (1): noopOpFormatter

### Community 22 - "Ginkgo Test Suites"
Cohesion: 0.67
Nodes (0):

### Community 23 - "Run Error Type"
Cohesion: 0.67
Nodes (1): RunError

### Community 24 - "CLI Main Entrypoint"
Cohesion: 1.0
Nodes (0):

### Community 25 - "Spinner Tests"
Cohesion: 1.0
Nodes (0):

### Community 26 - "Sudo Elevation Guard"
Cohesion: 1.0
Nodes (0):

### Community 27 - "PID Path Helper"
Cohesion: 1.0
Nodes (0):

### Community 28 - "Process Probe"
Cohesion: 1.0
Nodes (0):

### Community 29 - "PID Reader"
Cohesion: 1.0
Nodes (0):

### Community 30 - "PID Lock Writer"
Cohesion: 1.0
Nodes (0):

### Community 31 - "Run Error Tests"
Cohesion: 1.0
Nodes (0):

### Community 32 - "Op Ref Formatter"
Cohesion: 1.0
Nodes (0):

### Community 33 - "Generated Test Doubles"
Cohesion: 1.0
Nodes (2): Generated InputSourcer Test Double, Generated InputSrc Test Double

### Community 34 - "Tool Dependencies"
Cohesion: 1.0
Nodes (0):

### Community 35 - "CLI Integration Test File"
Cohesion: 1.0
Nodes (0):

### Community 36 - "Node Kill Tests"
Cohesion: 1.0
Nodes (0):

### Community 37 - "Local Node Tests"
Cohesion: 1.0
Nodes (0):

### Community 38 - "Node Kill Helper"
Cohesion: 1.0
Nodes (0):

### Community 39 - "Node Create Tests"
Cohesion: 1.0
Nodes (0):

### Community 40 - "Data Resolver Tests"
Cohesion: 1.0
Nodes (0):

### Community 41 - "CliColorer Tests"
Cohesion: 1.0
Nodes (0):

### Community 42 - "InputSourcer Tests"
Cohesion: 1.0
Nodes (0):

### Community 43 - "Input Factory Tests"
Cohesion: 1.0
Nodes (0):

### Community 44 - "Env Var Tests"
Cohesion: 1.0
Nodes (0):

### Community 45 - "Param Default Tests"
Cohesion: 1.0
Nodes (0):

### Community 46 - "Slice Input Tests"
Cohesion: 1.0
Nodes (0):

### Community 47 - "YAML Input Tests"
Cohesion: 1.0
Nodes (0):

### Community 48 - "Run Error Exit Code"
Cohesion: 1.0
Nodes (1): Run Error Exit Code

## Knowledge Gaps
- **44 isolated node(s):** `NodeProvider`, `NodeConfig`, `LoadingSpinner`, `InputSrc`, `CLI for opctl` (+39 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `CLI Main Entrypoint`** (2 nodes): `main.go`, `main()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Spinner Tests`** (2 nodes): `loading_test.go`, `TestDotLoadingSpinner()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Sudo Elevation Guard`** (2 nodes): `ensure.go`, `Ensure()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `PID Path Helper`** (2 nodes): `constructPIDFilePath.go`, `constructPIDFilePath()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Process Probe`** (2 nodes): `tryGetProcess.go`, `TryGetProcess()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `PID Reader`** (2 nodes): `getPIDFromFile.go`, `getPIDFromFile()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `PID Lock Writer`** (2 nodes): `tryGetLock.go`, `TryGetLock()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Run Error Tests`** (2 nodes): `runError_test.go`, `TestRunError()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Op Ref Formatter`** (2 nodes): `formatOpRef.go`, `FormatOpRef()`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Generated Test Doubles`** (2 nodes): `Generated InputSourcer Test Double`, `Generated InputSrc Test Double`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Tool Dependencies`** (1 nodes): `tools.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `CLI Integration Test File`** (1 nodes): `cli_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Node Kill Tests`** (1 nodes): `killNodeIfExists_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Local Node Tests`** (1 nodes): `local_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Node Kill Helper`** (1 nodes): `killNodeIfExists.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Node Create Tests`** (1 nodes): `createNodeIfNotExists_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Data Resolver Tests`** (1 nodes): `dataResolver_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `CliColorer Tests`** (1 nodes): `cliColorer_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `InputSourcer Tests`** (1 nodes): `inputSourcer_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Input Factory Tests`** (1 nodes): `inputSrcFactory_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Env Var Tests`** (1 nodes): `envVar_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Param Default Tests`** (1 nodes): `paramDefault_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Slice Input Tests`** (1 nodes): `slice_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `YAML Input Tests`** (1 nodes): `ymlFile_test.go`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.
- **Thin community `Run Error Exit Code`** (1 nodes): `Run Error Exit Code`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `FakeCLIParamSatisfier` connect `Fake Satisfier Harness` to `Command Wiring Layer`?**
  _High betweenness centrality (0.082) - this node is a cross-community bridge._
- **Why does `FakeCliColorer` connect `Fake Colorer Harness` to `Command Wiring Layer`?**
  _High betweenness centrality (0.078) - this node is a cross-community bridge._
- **Why does `CliOutput` connect `CliOutput Methods` to `Command Wiring Layer`?**
  _High betweenness centrality (0.032) - this node is a cross-community bridge._
- **What connects `NodeProvider`, `NodeConfig`, `LoadingSpinner` to the rest of the system?**
  _44 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Command Wiring Layer` be split into smaller, more focused modules?**
  _Cohesion score 0.04 - nodes in this community are weakly interconnected._
- **Should `Fake Satisfier Harness` be split into smaller, more focused modules?**
  _Cohesion score 0.06 - nodes in this community are weakly interconnected._
- **Should `Fake Colorer Harness` be split into smaller, more focused modules?**
  _Cohesion score 0.07 - nodes in this community are weakly interconnected._