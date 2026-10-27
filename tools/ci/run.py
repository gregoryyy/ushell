#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.13"
# dependencies = ["pyyaml==6.0.2"]
#
# [tool.uv]
# exclude-newer = "2026-10-12T00:00:00Z"
# ///
"""Local CI (tools/ci/README.md): runs the jobs of .github/workflows/ci.yml on this machine while
GitHub Actions doesn't start jobs. The run takes the workflow from the commit itself and runs it in
a clean worktree of that commit, so uncommitted changes can neither make it pass nor break it. Jobs
run one after another; matrix entries for another platform than this machine's are not run. Logs
and the summary go to .ci/<date>-<commit>/, ignored by Git. One run at a time, across clones; a
second one waits for the first. Adapted from ../lakeshore/utils/ci/run.py.

    tools/ci/run.py [rev] [--job links,ush] [--post] [--keep]

--post sets a commit status per job that ran on GitHub, `local / <job name>`: pending first, then
the result, and `error` for jobs a stopped run didn't finish. The PR shows them next to (not in
place of) the Actions checks, which fail while Actions doesn't start jobs. Statuses use the GitHub
API, not Actions; the token stays with `gh`. --keep keeps the worktree. Exits 1 if a job fails.
"""

import argparse
import fcntl
import os
import platform
import shutil
import signal
import subprocess
import sys
import tempfile
import time
from collections.abc import Callable, Generator
from contextlib import contextmanager
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Literal

from workflow import Job, LocalStep, WorkflowError, jobs_of, parse, runs_here

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ".github/workflows/ci.yml"
STATE = ROOT / ".ci"
# Free space a run needs for the worktree, virtual environments and output.
MIN_FREE_GB = 1.0
# The environment steps see: everything else (tokens, API keys) stays out, as on a hosted runner.
KEEP_ENV = ("PATH", "HOME", "USER", "LOGNAME", "LANG", "LC_ALL", "TMPDIR")
# Commands CI never runs here: live models (tests use fake providers) and the real sudo (the zsh
# plugin tests use a fake one; a real sudo prompts or logs failed authentications).
BLOCKED = ("claude", "codex", "sudo")

State = Literal["pending", "success", "failure", "error"]


def git(*args: str, cwd: Path = ROOT) -> str:
    return subprocess.run(
        ["git", *args], cwd=cwd, check=True, capture_output=True, text=True
    ).stdout.strip()


def duration(seconds: float) -> str:
    s = round(seconds)
    return f"{s} s" if s < 60 else f"{s // 60} min {s % 60:02d} s"


def post(sha: str, job: Job, state: State, description: str) -> None:
    """Sets the commit status `local / <job name>` on GitHub through `gh`, which holds the token."""
    subprocess.run(
        [
            "gh",
            "api",
            "--method",
            "POST",
            f"repos/{{owner}}/{{repo}}/statuses/{sha}",
            "-f",
            f"state={state}",
            "-f",
            f"context=local / {job.name}",
            "-f",
            f"description={description[:140]}",
        ],
        cwd=ROOT,
        check=True,
        capture_output=True,
        text=True,
    )


@contextmanager
def one_run_at_a_time() -> Generator[None]:
    """Runs wait for each other, from any clone: the lock is in the temporary directory, not in
    the clone's .ci/."""
    with (Path(tempfile.gettempdir()) / "ushell-ci.lock").open("w") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            print("Another local CI run is in progress; waiting for it.", flush=True)
            fcntl.flock(lock, fcntl.LOCK_EX)
        yield


def shell(command: str, cwd: Path, env: dict[str, str], log: Path, deadline: float) -> bool:
    """Runs a command as GitHub runs `run` steps (bash, -e, pipefail), output appended to the log.

    The command gets its own process group, so a timeout or Ctrl-C stops everything it started.
    """
    with log.open("a", encoding="utf-8") as out:
        child = subprocess.Popen(
            ["bash", "--noprofile", "--norc", "-eo", "pipefail", "-c", command],
            cwd=cwd,
            env=env,
            stdin=subprocess.DEVNULL,
            stdout=out,
            stderr=subprocess.STDOUT,
            start_new_session=True,
        )
        try:
            return child.wait(timeout=max(1.0, deadline - time.monotonic())) == 0
        except subprocess.TimeoutExpired:
            out.write("\n[local CI] timeout-minutes reached\n")
            return False
        finally:
            if child.poll() is None:
                stop(child)


def stop(child: subprocess.Popen[bytes]) -> None:
    for sig in (signal.SIGTERM, signal.SIGKILL):
        try:
            os.killpg(child.pid, sig)
        except ProcessLookupError:
            return
        try:
            child.wait(timeout=5)
            return
        except subprocess.TimeoutExpired:
            continue


def tail(log: Path, start: int, n: int = 30) -> str:
    text = log.read_bytes()[start:].decode("utf-8", "replace").rstrip()
    return "\n".join(text.splitlines()[-n:])


@dataclass
class Outcome:
    job: Job
    ok: bool = True
    seconds: float = 0.0
    # (label, pass | fail | skip, note, seconds)
    steps: list[tuple[str, str, str, float]] = field(
        default_factory=list[tuple[str, str, str, float]]
    )


@dataclass
class Context:
    sha: str
    worktree: Path
    out: Path
    tmp: Path


def run_step(
    step: LocalStep, ctx: Context, env: dict[str, str], bin_dir: Path, log: Path, deadline: float
) -> tuple[bool, str]:
    def note(text: str) -> None:
        with log.open("a", encoding="utf-8") as f:
            f.write(text + "\n")

    match step.kind:
        case "checkout":
            note(f"worktree of {ctx.sha} at {ctx.worktree}")
            return True, ""
        case "setup-uv":
            if step.python:
                env["UV_PYTHON"] = step.python
            note(f"uv from PATH; UV_PYTHON={step.python or 'unset'}")
            return True, ""
        case "setup-python":
            return setup_python(step.python, env, bin_dir, ctx.tmp, note)
        case "secrets":
            if shutil.which("gitleaks") is None:
                note("gitleaks is not installed: brew install gitleaks")
                return False, "gitleaks missing"
            # The whole history up to the commit: more than the action scans on a PR, still fast.
            command = f"gitleaks git --no-banner --redact --log-opts={ctx.sha} ."
            return shell(command, ctx.worktree, env, log, deadline), ""
        case _:  # shell
            note(f"$ {step.command}   (in {step.cwd})")
            return shell(step.command, ctx.worktree / step.cwd, env | step.env, log, deadline), ""


def setup_python(
    wanted: str | None, env: dict[str, str], bin_dir: Path, tmp: Path, note: Callable[[str], None]
) -> tuple[bool, str]:
    """A Python of the asked version, found by uv among the installed ones, becomes `python` and
    `python3` first on PATH. The patch version is this machine's, not GitHub's latest."""
    if shutil.which("uv", path=env.get("PATH")) is None:
        note("uv is not installed (brew install uv); it finds the Python for setup-python")
        return False, "uv missing"
    request = wanted or "3"
    found = subprocess.run(
        ["uv", "python", "find", "--no-project", request],
        cwd=tmp,  # no .python-version of the caller's directory
        env=env | {"UV_PYTHON_DOWNLOADS": "never"},
        capture_output=True,
        text=True,
        check=False,
    )
    if found.returncode != 0:
        note(f"no Python {request} installed: uv python install {request}")
        return False, f"Python {request} missing"
    # The interpreter itself, not a shim: a pyenv shim linked as `python` would run another one.
    probe = "import platform, sys; print(sys.executable); print(platform.python_version())"
    executable, version = subprocess.run(
        [found.stdout.strip(), "-c", probe], capture_output=True, text=True, check=True
    ).stdout.split()
    for name in ("python", "python3"):
        link = bin_dir / name
        link.unlink(missing_ok=True)
        link.symlink_to(executable)
    note(f"Python {version} at {executable}; workflow asks for {request}")
    return True, f"Python {version}"


def blocked_shims(bin_dir: Path) -> None:
    for name in BLOCKED:
        path = bin_dir / name
        path.write_text(f"#!/bin/sh\necho 'local CI: {name} is not available in CI' >&2\nexit 1\n")
        path.chmod(0o755)


def run_job(job: Job, ctx: Context) -> Outcome:
    started = time.monotonic()
    deadline = started + 60 * job.timeout_minutes
    log = ctx.out / f"{job.key}.log"
    summary = ctx.out / f"{job.key}.summary.md"
    log.write_text(f"# {job.name}\n# commit {ctx.sha}\n", encoding="utf-8")
    runner_temp = Path(tempfile.mkdtemp(prefix=f"{job.key}-", dir=ctx.tmp))
    # Per job: setup-python links its interpreter here, next to the blocked commands.
    bin_dir = runner_temp / "bin"
    bin_dir.mkdir()
    blocked_shims(bin_dir)
    env = {k: os.environ[k] for k in KEEP_ENV if k in os.environ}
    env["PATH"] = f"{bin_dir}:{env.get('PATH', '')}"
    env |= {
        "CI": "true",
        "RUNNER_TEMP": str(runner_temp),
        "RUNNER_OS": platform.system(),
        "GITHUB_WORKSPACE": str(ctx.worktree),
        "GITHUB_SHA": ctx.sha,
        "GITHUB_STEP_SUMMARY": str(summary),
    }
    env |= job.env
    outcome = Outcome(job)
    for step in job.steps:
        if (step.when == "success" and not outcome.ok) or (step.when == "failure" and outcome.ok):
            outcome.steps.append((step.label, "skip", "", 0.0))
            continue
        t = time.monotonic()
        start = log.stat().st_size
        with log.open("a", encoding="utf-8") as f:
            f.write(f"\n## {step.label}\n")
        ok, note = run_step(step, ctx, env, bin_dir, log, deadline)
        seconds = time.monotonic() - t
        outcome.steps.append((step.label, "pass" if ok else "fail", note, seconds))
        extra = f"  ({note})" if note else ""
        print(f"  {'pass' if ok else 'fail'}  {step.label}{extra}  {duration(seconds)}", flush=True)
        if not ok:
            outcome.ok = False
            print("\n".join("        " + line for line in tail(log, start).splitlines()))
            print(f"        … full log: {log.relative_to(ROOT)}", flush=True)
    outcome.seconds = time.monotonic() - started
    return outcome


def write_summary(
    ctx: Context, outcomes: list[Outcome], elsewhere: list[Job], when: datetime
) -> Path:
    host = f"{platform.system()} {platform.machine()}"
    lines = [
        f"# Local CI: {ctx.sha[:7]}",
        "",
        f"Commit {ctx.sha}, run {when:%Y-%m-%d %H:%M} on {host}. Workflow: {WORKFLOW} at that "
        "commit.",
        "",
        "| Job | Result | Time |",
        "| --- | --- | --- |",
        *(
            f"| {o.job.name} | {'pass' if o.ok else 'fail'} | {duration(o.seconds)} |"
            for o in outcomes
        ),
        *(f"| {j.name} | not run here ({j.runs_on}) | |" for j in elsewhere),
        "",
    ]
    for o in outcomes:
        lines += [f"## {o.job.name}", ""]
        lines += [f"- {r}: {label}{f' ({n})' if n else ''}" for label, r, n, _ in o.steps]
        lines.append("")
        summary = ctx.out / f"{o.job.key}.summary.md"
        if summary.exists():
            lines += [f"## Summary of {o.job.name}", "", summary.read_text(encoding="utf-8"), ""]
    path = ctx.out / "summary.md"
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return path


def main() -> int:
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument("rev", nargs="?", default="HEAD")
    parser.add_argument("--job", help="comma-separated job IDs (default: all)")
    parser.add_argument("--post", action="store_true", help="commit statuses on GitHub")
    parser.add_argument("--keep", action="store_true", help="keep the worktree")
    args = parser.parse_args()

    sha = git("rev-parse", "--verify", f"{args.rev}^{{commit}}")
    short = sha[:7]
    every = jobs_of(parse(git("show", f"{sha}:{WORKFLOW}")))  # every step mapped, or an error
    wanted = [j for j in (args.job or "").split(",") if j]
    if unknown := [j for j in wanted if j not in {x.id for x in every}]:
        ids = dict.fromkeys(x.id for x in every)
        raise WorkflowError(f"no job {', '.join(unknown)}; the jobs are {', '.join(ids)}")
    selected = [j for j in every if not wanted or j.id in wanted]
    system = platform.system()
    jobs = [j for j in selected if runs_here(j, system)]
    elsewhere = [j for j in selected if not runs_here(j, system)]
    free = shutil.disk_usage(tempfile.gettempdir()).free / 2**30
    if free < MIN_FREE_GB:
        raise RuntimeError(f"{free:.1f} GB free; a run needs about {MIN_FREE_GB} GB")
    if sha == git("rev-parse", "HEAD") and git("status", "--porcelain", "--untracked-files=no"):
        print("Note: uncommitted changes are not part of this run; it checks the commit.")
    if args.post and not git("branch", "-r", "--contains", sha):
        raise RuntimeError(f"--post: {short} is not on GitHub yet; push it first")

    with one_run_at_a_time():
        if args.post:
            for job in jobs:
                post(sha, job, "pending", "Local CI (just ci) running")
        when = datetime.now()
        out = STATE / f"{when:%Y-%m-%d-%H%M}-{short}"
        out.mkdir(parents=True, exist_ok=True)
        tmp = Path(tempfile.mkdtemp(prefix=f"ushell-ci-{short}-"))
        worktree = tmp / "worktree"
        git("worktree", "add", "--detach", "--quiet", str(worktree), sha)
        ctx = Context(sha, worktree, out, tmp)
        outcomes: list[Outcome] = []
        post_failed = False
        subject = git("log", "-1", "--format=%s", sha)
        print(
            f"Local CI for {short} ({subject}), {len(jobs)} job(s), logs in {out.relative_to(ROOT)}"
        )
        for job in elsewhere:
            print(f"Not run on {system}: {job.name}")
        try:
            for job in jobs:
                print(f"\n{job.name}", flush=True)
                o = run_job(job, ctx)
                outcomes.append(o)
                print(f"  → {'pass' if o.ok else 'fail'} in {duration(o.seconds)}", flush=True)
                if args.post:
                    try:
                        verdict = "passed" if o.ok else "failed"
                        description = f"Local CI (just ci): {verdict} in {duration(o.seconds)}"
                        post(sha, job, "success" if o.ok else "failure", description)
                    except subprocess.CalledProcessError as e:
                        post_failed = True
                        print(f"  could not post the status: {e.stderr.strip()}", file=sys.stderr)
        except BaseException as e:
            # A run that stops early leaves no status pending: unfinished jobs get `error`.
            if args.post:
                reason = "interrupted" if isinstance(e, KeyboardInterrupt) else str(e)
                done = {o.job.key for o in outcomes}
                for job in jobs:
                    if job.key not in done:
                        try:
                            post(sha, job, "error", f"Local CI stopped: {reason}")
                        except subprocess.CalledProcessError:
                            pass
            raise
        finally:
            if not args.keep:
                git("worktree", "remove", "--force", str(worktree))
                shutil.rmtree(tmp, ignore_errors=True)

        summary = write_summary(ctx, outcomes, elsewhere, when)
    print()
    for o in outcomes:
        print(f"{o.job.name}: {'pass' if o.ok else 'fail'}")
    posted = (
        ("; posting a status failed" if post_failed else "; statuses posted") if args.post else ""
    )
    print(f"Summary: {summary.relative_to(ROOT)}{posted}")
    return 0 if all(o.ok for o in outcomes) and not post_failed else 1


def interrupt(_signum: int, _frame: object) -> None:
    raise KeyboardInterrupt


if __name__ == "__main__":
    # Ctrl-C and a kill stop the run the same way: steps stopped, the worktree removed, unfinished
    # statuses set to error. SIGINT is set explicitly; a background start ignores it.
    signal.signal(signal.SIGINT, interrupt)
    signal.signal(signal.SIGTERM, interrupt)
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        sys.exit(130)
    except (WorkflowError, RuntimeError, subprocess.CalledProcessError) as e:
        detail = getattr(e, "stderr", "") or ""
        print(f"ci: {e} {detail}".strip(), file=sys.stderr)
        sys.exit(1)
