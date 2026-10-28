from pathlib import Path

import pytest

from workflow import WorkflowError, condition, go_matches, jobs_of, matrix, parse, runs_here

CI = Path(__file__).resolve().parents[2] / ".github" / "workflows" / "ci.yml"
SETUP = "      - uses: actions/setup-python@v7"


def wf(steps: str, extra: str = "", job: str = "") -> str:
    return f"""
name: t
on:
  pull_request:
{extra}
jobs:
  a:
    runs-on: ubuntu-24.04
{job}
    steps:
{steps}
"""


def test_the_repository_workflow_maps_completely() -> None:
    jobs = jobs_of(parse(CI.read_text(encoding="utf-8")))
    assert [j.key for j in jobs] == [
        "links",
        "ushpy-ubuntu-latest-3.10",
        "ushpy-ubuntu-latest-3.13",
        "ushpy-macos-latest-3.10",
        "ushpy-macos-latest-3.13",
        "ushgo-ubuntu-latest",
        "ushgo-macos-latest",
        "tools",
        "secrets",
    ]
    by_key = {j.key: j for j in jobs}
    ush = by_key["ushpy-macos-latest-3.10"]
    assert ush.name == "ushpy (macos-latest, 3.10)" and ush.runs_on == "macos-latest"
    assert [s.kind for s in ush.steps] == ["checkout", "setup-python", "shell", "shell"]
    assert ush.steps[1].python == "3.10" and ush.steps[3].cwd == "ushpy"
    assert [s.kind for s in by_key["secrets"].steps] == ["checkout", "secrets"]
    ushgo = by_key["ushgo-macos-latest"]
    assert ushgo.name == "ushgo (macos-latest)"
    assert [s.kind for s in ushgo.steps] == ["checkout", "setup-go", "shell"]
    assert ushgo.steps[1].go == "1.27" and ushgo.steps[2].cwd == "ushgo"
    tools = by_key["tools"]
    assert tools.steps[1].kind == "setup-uv" and tools.steps[1].python == "3.13"
    assert all(s.cwd == "tools/ci" for s in tools.steps if s.kind == "shell")


def test_only_this_platforms_matrix_entries_run_here() -> None:
    jobs = jobs_of(parse(CI.read_text(encoding="utf-8")))
    here = [j.key for j in jobs if runs_here(j, "Darwin")]
    assert here == [
        "links",
        "ushpy-macos-latest-3.10",
        "ushpy-macos-latest-3.13",
        "ushgo-macos-latest",
        "tools",
        "secrets",
    ]
    on_linux = [j.key for j in jobs if runs_here(j, "Linux")]
    assert "ushpy-ubuntu-latest-3.10" in on_linux and "ushpy-macos-latest-3.10" not in on_linux


def test_an_unknown_matrix_runner_is_an_error() -> None:
    text = wf("      - run: echo", job="    strategy: {matrix: {os: [solaris-11]}}").replace(
        "runs-on: ubuntu-24.04", "runs-on: ${{ matrix.os }}"
    )
    (job,) = jobs_of(parse(text))
    with pytest.raises(WorkflowError, match="unknown runner"):
        runs_here(job, "Darwin")


def test_matrix_order_names_and_values_in_with() -> None:
    steps = (
        "      - uses: actions/setup-python@v7\n"
        "        name: Python ${{ matrix.py }}\n"
        "        with: {python-version: '${{ matrix.py }}'}"
    )
    job = "    strategy: {matrix: {os: [x, y], py: ['3.10', '3.13']}}"
    jobs = jobs_of(parse(wf(steps, job=job)))
    assert [j.name for j in jobs] == ["a (x, 3.10)", "a (x, 3.13)", "a (y, 3.10)", "a (y, 3.13)"]
    assert jobs[1].steps[0].python == "3.13" and jobs[1].steps[0].label == "Python 3.13"
    assert not jobs[0].matrix_runner and runs_here(jobs[0], "Windows")
    named = jobs_of(parse(wf(steps, job=job + "\n    name: build on ${{ matrix.os }}")))
    assert named[2].name == "build on y"


def test_matrix_shapes() -> None:
    assert matrix(None, "s") == [{}]
    assert matrix({"fail-fast": False, "matrix": {"a": [1, 2]}}, "s") == [{"a": "1"}, {"a": "2"}]
    with pytest.raises(WorkflowError, match="include isn't supported"):
        matrix({"matrix": {"a": [1], "include": [{"a": 2}]}}, "s")
    with pytest.raises(WorkflowError, match="list of values"):
        matrix({"matrix": {"a": 1}}, "s")
    with pytest.raises(WorkflowError, match="unsupported keys"):
        matrix({"max-parallel": 2}, "s")


def test_on_is_read_as_a_key_not_as_true() -> None:
    assert "on" in parse(wf("      - run: echo"))


def test_job_defaults_override_workflow_defaults() -> None:
    text = """
on: {pull_request: null}
defaults: {run: {working-directory: top}}
jobs:
  a:
    steps: [{run: echo}]
  b:
    defaults: {run: {working-directory: sub}}
    steps: [{run: echo}, {run: echo, working-directory: own}]
"""
    a, b = jobs_of(parse(text))
    assert a.steps[0].cwd == "top" and [s.cwd for s in b.steps] == ["sub", "own"]


def test_conditions() -> None:
    assert condition(None) == "success"
    assert condition("failure()") == "failure" and condition("${{ always() }}") == "always"
    with pytest.raises(WorkflowError, match="only success"):
        condition("runner.os == 'Linux'")


@pytest.mark.parametrize(
    ("steps", "extra", "job", "message"),
    [
        ("      - uses: actions/upload-artifact@v4", "", "", "no local equivalent"),
        ("      - run: echo ${{ github.sha }}", "", "", "expressions"),
        ("      - run: echo ${{ matrix.os }}", "", "", "expressions"),
        ("      - run: echo\n        env: {A: '${{ secrets.X }}'}", "", "", "expressions"),
        ("      - run: echo\n        shell: pwsh", "", "", "unsupported keys"),
        ("      - run: echo\n        uses: actions/checkout@v7", "", "", "exactly one"),
        ("      - run: echo", "container: ubuntu", "", "unsupported workflow keys"),
        ("      - run: echo", "defaults: {run: {shell: zsh}}", "", "unsupported keys"),
        ("      - run: echo", "", "    services: {db: {image: postgres}}", "unsupported keys"),
        (f"{SETUP}\n        with: {{python-version: '${{{{ matrix.v }}}}'}}", "", "", "no matrix"),
        (f"{SETUP}\n        with: {{python-version: '${{{{ env.V }}}}'}}", "", "", "expressions"),
    ],
)
def test_unsupported_constructs_are_errors(steps: str, extra: str, job: str, message: str) -> None:
    with pytest.raises(WorkflowError, match=message):
        jobs_of(parse(wf(steps, extra, job)))


def test_an_expression_in_an_action_steps_env_is_left_to_github() -> None:
    steps = (
        "      - uses: gitleaks/gitleaks-action@v3\n"
        "        env: {GITHUB_TOKEN: '${{ secrets.GITHUB_TOKEN }}'}"
    )
    (job,) = jobs_of(parse(wf(steps)))
    assert job.steps[0].kind == "secrets"


def test_go_versions_match_exactly_or_by_prefix() -> None:
    assert go_matches("1.27", "go1.27.3") and go_matches("1.27.3", "go1.27.3")
    assert go_matches(None, "go1.20") and go_matches("", "go1.27.3")
    assert not go_matches("1.27", "go1.26.9") and not go_matches("1.2", "go1.27.3")
    assert not go_matches("1.27", "")
