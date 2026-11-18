# Contributing

Thanks for helping make `os-pangolin` a reliable native Pangolin integration for
OPNsense.

## Ground rules

- The runtime must remain native FreeBSD/OPNsense. Do not add Linux VMs,
  containers, Docker, Linux compatibility layers, external Linux gateways or
  extra Linux subnet routers.
- Prefer official OPNsense mechanisms: MVC, Volt, configd, rc.d, the firewall
  plugin API, the Unbound include directory and the Unbound model.
- Never install a default route, never expose all VLANs, never flush the global
  pf ruleset and never edit or delete foreign Unbound entries.
- Keep upstream patches minimal and isolated. Put new platform code in new
  files and small dispatch changes; keep the diff reviewable for `fosrl/newt`
  and `fosrl/olm`.
- Do not commit secrets, live credentials, generated agent binaries or
  test-VM disks.
- Tests must be real. Do not claim a live tunnel, HA failover or hardware test
  unless you actually ran it.

## Development setup

Requirements:

- Go 1.26+ (1.27.4 used for the current build);
- PHP 8.x for linting;
- Python 3 for XML checks;
- Git;
- for integration: QEMU and an OPNsense 26.7 installer/nano image.

```sh
go version
cd agent && go test ./... && cd ..
sh scripts/lint.sh
sh scripts/build-agent.sh
```

The agent build uses the vendored `third_party/newt` and `third_party/olm`
through `replace` directives, so no network fetch of those two modules is
needed.

## Repository conventions

- `agent/` is the Go module and owns the runtime.
- `plugin/src/etc` maps to `/usr/local/etc`; `plugin/src/opnsense` maps to
  `/usr/local/opnsense`; `plugin/src/sbin` maps to `/usr/local/sbin`.
- OPNsense model fields use the `//OPNsense/pangolin` mount.
- UI field ids are `pangolin.<section>.<field>`.
- Generated files are never committed; `plugin/src/sbin/pangolin-agent` is
  built by `scripts/build-agent.sh`.
- Vendored upstream sources are pinned in `third_party/README.md`. Changes must
  also appear in `patches/` and be described in `docs/PORTING.md`.

## Coding style

Go:

- `gofmt`/`go vet` clean.
- Validate every value that reaches exec, pf, Unbound or a file path.
- Prefer explicit argument vectors over shell strings.
- Return errors; avoid `os.Exit` in new library code (upstream has some).
- Keep comments focused on why, especially around security invariants.

PHP/Volt:

- Follow OPNsense MVC patterns; use `BaseModel`, `ApiMutableModelControllerBase`
  and configd actions.
- Use `mwexecf`/`mwexecfm`/`mwexecfb`, never `mwexec` (undefined on OPNsense
  26.7), and never call `pluginctl`/`configctl` from inside a configd hook
  (nested configd calls deadlock).
- Escape output in Volt; never interpolate secrets into HTML.
- Keep generated files atomic and mode-correct.

## Tests

Minimum before a pull request:

```sh
cd agent && go test ./... && go vet ./...
GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build ./...
GOOS=freebsd GOARCH=arm64 CGO_ENABLED=0 go build ./...
cd ..
python3 scripts/check-php.py "$(command -v php)"
python3 scripts/check-xml.py
```

For behavior changes in pf/Unbound, add or update the QEMU fixtures in
`tests/integration/qemu/` and record the observed output in the PR. Do not
weaken validation to make a test pass.

## Commit messages

Use a short imperative subject and a body that explains the security or
compatibility impact:

```
agent: reject overlapping subnet-router networks

Prevents a remote route from overlapping a configured local network and
creating a forwarding loop. Adds unit coverage and updates the DHCP fixture.
```

## Upstream patches

1. Edit `third_party/<module>`.
2. Regenerate the diff with the command in `docs/PORTING.md`.
3. Run the FreeBSD build and tests.
4. Update `third_party/README.md`, `patches/`, and `docs/COMPATIBILITY.md`.
5. Keep unrelated formatting changes out of the patch.

## License

By contributing you agree that your contribution is licensed under AGPL-3.0.
