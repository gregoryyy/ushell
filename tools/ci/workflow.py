"""The jobs of a GitHub Actions workflow, read for the local CI (tools/ci/README.md). Pure.

The workflow file stays the one definition of CI: `run` steps run as written, and each `uses` step
maps to a local equivalent below. An action without one, an expression in a `run` step or its
`env`, a condition other than success(), failure() or always(), or a key this module doesn't know
is an error, so the two can't drift apart silently. Adapted from ../lakeshore/utils/ci/workflow.py;
this version adds `strategy.matrix` (lists only), whose values may appear as `${{ matrix.KEY }}` in
a job's `name` and `runs-on` and in a step's `name` and `with`.
"""

import itertools
import re
from dataclasses import dataclass, field
from typing import Any, Literal

import yaml

Condition = Literal["success", "failure", "always"]
Kind = Literal["shell", "checkout", "setup-uv", "setup-python", "setup-go", "secrets"]

# Allowed keys. `permissions`, `concurrency` and `runs-on` concern GitHub's runners and token only;
# a matrix of runners decides which entries run locally (`runs_here`).
WORKFLOW_KEYS = {"name", "on", "permissions", "concurrency", "defaults", "env", "jobs"}
JOB_KEYS = {
    "name",
    "runs-on",
    "timeout-minutes",
    "permissions",
    "defaults",
    "env",
    "strategy",
    "steps",
}
STEP_KEYS = {"name", "run", "uses", "with", "env", "if", "working-directory"}
STRATEGY_KEYS = {"matrix", "fail-fast"}
MATRIX_REF = re.compile(r"\$\{\{\s*matrix\.([A-Za-z_][\w-]*)\s*\}\}")
# Runner labels by platform.system() of the machine that can stand in for them.
PLATFORMS = {"ubuntu": "Linux", "macos": "Darwin", "windows": "Windows"}


class WorkflowError(ValueError):
    """The workflow uses something the local CI can't run."""


@dataclass(frozen=True)
class LocalStep:
    """A step as it runs locally. `cwd` is relative to the checkout."""

    label: str
    kind: Kind
    when: Condition = "success"
    command: str = ""
    cwd: str = "."
    env: dict[str, str] = field(default_factory=dict[str, str])
    # setup-uv and setup-python: the Python version the workflow asks for.
    python: str | None = None
    # setup-go: the Go version the workflow asks for.
    go: str | None = None


@dataclass(frozen=True)
class Job:
    """One job, or one entry of a job's matrix, its steps mapped to local ones.

    `id` is the workflow's job ID, shared by the entries of a matrix; `key` is unique per entry.
    `matrix_runner` is true when `runs-on` comes from the matrix, so the matrix picks the platform.
    """

    id: str
    key: str
    name: str
    runs_on: str
    matrix_runner: bool
    timeout_minutes: float
    env: dict[str, str]
    steps: tuple[LocalStep, ...]


def parse(text: str) -> dict[str, Any]:
    """The workflow as a mapping; YAML 1.1 reads the key `on` as True, so it's renamed back."""
    doc: Any = yaml.safe_load(text)
    if not isinstance(doc, dict):
        raise WorkflowError("a workflow is a mapping")
    out: dict[str, Any] = {("on" if k is True else str(k)): v for k, v in doc.items()}  # pyright: ignore[reportUnknownVariableType]
    if unknown := set(out) - WORKFLOW_KEYS:
        raise WorkflowError(f"unsupported workflow keys: {sorted(unknown)}")
    return out


def _map(value: Any, where: str) -> dict[str, Any]:
    if value is None:
        return {}
    if not isinstance(value, dict):
        raise WorkflowError(f"{where}: expected a mapping")
    return {str(k): v for k, v in value.items()}  # pyright: ignore[reportUnknownVariableType]


def _strings(value: Any, where: str) -> dict[str, str]:
    return {k: str(v) for k, v in _map(value, where).items()}


def _known(keys: dict[str, Any], allowed: set[str], where: str) -> None:
    if unknown := set(keys) - allowed:
        raise WorkflowError(f"{where}: unsupported keys {sorted(unknown)}")


def _no_expressions(value: dict[str, str] | str, where: str) -> None:
    texts = value.values() if isinstance(value, dict) else [value]
    if any("${{" in t for t in texts):
        raise WorkflowError(f"{where}: expressions aren't supported locally")


def _fill(text: str, values: dict[str, str], where: str) -> str:
    """`${{ matrix.KEY }}` replaced by the entry's value; any other expression is an error."""

    def value(m: re.Match[str]) -> str:
        if m.group(1) not in values:
            raise WorkflowError(f"{where}: no matrix value {m.group(1)!r}")
        return values[m.group(1)]

    out = MATRIX_REF.sub(value, text)
    _no_expressions(out, where)
    return out


def condition(expr: str | None) -> Condition:
    """When a step runs: GitHub's status functions, the only conditions supported."""
    if expr is None:
        return "success"
    m = re.fullmatch(r"(?:\$\{\{\s*)?(success|failure|always)\(\)(?:\s*\}\})?", expr.strip())
    if not m:
        raise WorkflowError(f"if: {expr!r}: only success(), failure() and always() are supported")
    return m.group(1)  # type: ignore[return-value]


def _run_defaults(value: Any, where: str) -> dict[str, str]:
    run = _strings(_map(value, where).get("run"), f"{where}.run")
    _known(run, {"working-directory"}, f"{where}.run")
    return run


def matrix(value: Any, where: str) -> list[dict[str, str]]:
    """The entries of `strategy.matrix`, in GitHub's order (the first key varies slowest)."""
    strategy = _map(value, where)
    _known(strategy, STRATEGY_KEYS, where)
    axes = _map(strategy.get("matrix"), f"{where}.matrix")
    if not axes:
        return [{}]
    for key, values in axes.items():
        if key in ("include", "exclude"):
            raise WorkflowError(f"{where}.matrix: {key} isn't supported locally")
        if not isinstance(values, list) or not values:
            raise WorkflowError(f"{where}.matrix.{key}: expected a list of values")
    lists: list[list[str]] = [[str(v) for v in vs] for vs in axes.values()]  # pyright: ignore[reportUnknownVariableType, reportUnknownArgumentType]
    return [dict(zip(axes, combo, strict=True)) for combo in itertools.product(*lists)]


def _step(raw: Any, where: str, workdir: str, values: dict[str, str]) -> LocalStep:
    s = _map(raw, where)
    _known(s, STEP_KEYS, where)
    if ("run" in s) == ("uses" in s):
        raise WorkflowError(f"{where}: needs exactly one of run and uses")
    when = condition(s.get("if"))
    env = _strings(s.get("env"), f"{where}.env")
    name = _fill(str(s["name"]), values, f"{where}.name") if s.get("name") else None
    if "run" in s:
        command = str(s["run"])
        _no_expressions(command, where)
        _no_expressions(env, f"{where}.env")
        cwd = str(s.get("working-directory", workdir))
        label = name or command.splitlines()[0]
        return LocalStep(label, "shell", when, command=command, cwd=cwd, env=env)
    uses = str(s["uses"])
    action = uses.split("@", 1)[0]
    label = name or action
    with_ = {
        k: _fill(v, values, f"{where}.with.{k}")
        for k, v in _strings(s.get("with"), f"{where}.with").items()
    }
    match action:
        # The run's clean worktree of the commit is the checkout.
        case "actions/checkout":
            return LocalStep(label, "checkout", when)
        # The machine's uv, with the Python version the workflow asks for.
        case "astral-sh/setup-uv":
            return LocalStep(label, "setup-uv", when, python=with_.get("python-version"))
        # A Python of that version found by uv, first on PATH as python and python3.
        case "actions/setup-python":
            return LocalStep(label, "setup-python", when, python=with_.get("python-version"))
        # The machine's Go, if its version is the one the workflow asks for.
        case "actions/setup-go":
            return LocalStep(label, "setup-go", when, go=with_.get("go-version"))
        # The gitleaks binary over the history up to the commit.
        case "gitleaks/gitleaks-action":
            return LocalStep(label, "secrets", when)
        case _:
            raise WorkflowError(f"{where}: no local equivalent for {action}; add one here")


def jobs_of(workflow: dict[str, Any]) -> list[Job]:
    """The jobs of a parsed workflow in file order, one per matrix entry, every step mapped."""
    jobs = _map(workflow.get("jobs"), "jobs")
    if not jobs:
        raise WorkflowError("the workflow has no jobs")
    wf_env = _strings(workflow.get("env"), "env")
    _no_expressions(wf_env, "env")
    wf_defaults = _run_defaults(workflow.get("defaults"), "defaults")
    out: list[Job] = []
    for jid, raw in jobs.items():
        where = f"jobs.{jid}"
        j = _map(raw, where)
        _known(j, JOB_KEYS, where)
        workdir = (wf_defaults | _run_defaults(j.get("defaults"), f"{where}.defaults")).get(
            "working-directory", "."
        )
        env = _strings(j.get("env"), f"{where}.env")
        _no_expressions(env, f"{where}.env")
        steps = j.get("steps")
        if not isinstance(steps, list) or not steps:
            raise WorkflowError(f"{where}.steps: expected a list of steps")
        raw_name = str(j.get("name") or jid)
        raw_runs_on = str(j.get("runs-on") or "")
        for values in matrix(j.get("strategy"), f"{where}.strategy"):
            name = _fill(raw_name, values, f"{where}.name")
            # GitHub names a matrix entry `<name> (<values>)` unless the name uses the matrix.
            if values and not MATRIX_REF.search(raw_name):
                name = f"{name} ({', '.join(values.values())})"
            out.append(
                Job(
                    id=jid,
                    key="-".join([jid, *values.values()]),
                    name=name,
                    runs_on=_fill(raw_runs_on, values, f"{where}.runs-on"),
                    matrix_runner=bool(MATRIX_REF.search(raw_runs_on)),
                    timeout_minutes=float(j.get("timeout-minutes", 360)),
                    env=wf_env | env,
                    steps=tuple(
                        _step(s, f"{where}.steps[{i}]", workdir, values)
                        for i, s in enumerate(steps)  # pyright: ignore[reportUnknownArgumentType, reportUnknownVariableType]
                    ),
                )
            )
    return out


def runs_here(job: Job, system: str) -> bool:
    """Whether the job runs on a machine whose platform.system() is `system`.

    A job with a fixed runner runs anywhere: this machine stands in for GitHub's. A matrix of
    runners picks platforms on purpose, so only the entries for this machine's platform run; the
    others would repeat them here and prove nothing about their own platform.
    """
    if not job.matrix_runner:
        return True
    family = job.runs_on.split("-", 1)[0]
    if family not in PLATFORMS:
        raise WorkflowError(f"jobs.{job.id}: unknown runner {job.runs_on!r}")
    return PLATFORMS[family] == system


def go_matches(wanted: str | None, goversion: str) -> bool:
    """Whether an installed Go (`go env GOVERSION`, e.g. `go1.27.3`) satisfies a setup-go
    `go-version` given as an exact version or a prefix (`1.27`). No version asks for any Go."""
    installed = goversion.removeprefix("go")
    return not wanted or installed == wanted or installed.startswith(wanted + ".")
