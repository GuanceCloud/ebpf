# DataKit maintenance branch

The `datakit-v0.11` branch is based on upstream `v0.11.0`
(`270c859894bd38cdd0c7783317b16343409e4df8`). The module path remains
`github.com/cilium/ebpf`, so consumers can use a Go module replacement.
Version `v0.11.0-dk.1` contains the existing DataKit reader corrections without
upgrading the loader, BTF implementation, or public map APIs.

## Patch scope

- Read auxv entries using native pointer width for i386 startup.
- Extend native i386 perf head counters across 32-bit wraparound, including
  forward and overwritable readers.
- Discover newly online CPUs and replace detached perf events when
  `ReaderOptions.CPURefreshInterval` is enabled. Drain retired buffers before
  unmapping them. `CPUChangeHandler` reports a coverage transition; it does not
  invent an exact number of lost events. Its callback must not block or call
  reader methods.
- Add `Reader.ReadBatch` to reuse caller-owned sample buffers and amortize reader
  locking. The first record may block; subsequent records drain already
  notified rings. Deliver a nonempty partial batch before handling its error.

These implementation files match the corresponding DataKit vendor patches.
Changes to collector aggregation or exported telemetry are outside this fork.

## Validation

The fork includes native-width auxv, simulated i386 counter wraparound,
detached-event and retired-buffer draining regressions. It also contains the
native perf batch regression for complete delivery, sparse batches and closing
a blocked reader. That test skips when the kernel denies BPF map creation.

The `DataKit patch regressions` workflow runs the focused regressions for amd64
and 386, the amd64 race detector, and an arm64 build. These checks do not replace
native i386 kernel testing or CPU offline/online acceptance in isolated VMs.
The upstream project does not promise support for 32-bit architectures; this
branch carries the DataKit-specific maintenance responsibility.

## Releases and updates

Publish an immutable `v0.11.0-dk.N` tag after regression checks pass. Consumers
must pin a tag or commit, never a moving branch. Update their module replacement,
module checksums and generated vendor metadata together. Re-running
`go mod vendor` must reproduce the implementation files without manual edits.
Go intentionally excludes dependency test files from vendor output; run this
repository's regressions separately.

Keep production patches in this branch, with tests explaining each behavior.
When upgrading the upstream baseline, compare each patch with the new upstream
implementation and remove an equivalent patch only after its regression passes.
New upstream versions already use a different auxv implementation. Perf CPU
coverage, counter-width and API behavior still require explicit verification.
