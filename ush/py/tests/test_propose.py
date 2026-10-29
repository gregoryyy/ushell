"""Propose mode (TODO 2.17): ush proposes, the user runs, `//` continues with what ran."""

import json
import os
import time

from helpers import SYNTHETIC_KEY, TempDirTest, fake_identity, text
from test_agent import assert_every_tool_use_answered
from ush.agent import PROPOSE_TOOL
from ush.handoff import (EXPLANATIONS_KEPT, SESSION_TTL, Explanations, Ran, Sessions,
                         handoff_dir, session_key, take_ran, write_proposal)


def propose_use(command: str, effect: str = "read", interactive: bool = False,
                ident: str = "p1", purpose: str = "test") -> dict:
    return {"type": "tool_use", "id": ident, "name": "propose_command",
            "input": {"command": command, "purpose": purpose, "effect": effect,
                      "interactive": interactive}}


class ProposeAgentTest(TempDirTest):
    def make(self, turns):
        return self.make_agent(turns, propose=True)

    def test_a_proposal_is_shown_and_not_run(self):
        made = os.path.join(self.tmp, "made")
        agent, provider, ui, history = self.make([
            [text("This creates the file."), propose_use(f"touch {made}", effect="change")],
        ])
        entry = agent.ask("make a file")
        self.assertEqual(entry.outcome, "proposed")
        self.assertFalse(os.path.exists(made))
        self.assertEqual(len(provider.requests), 1)
        self.assertEqual(agent.proposal["command"], f"touch {made}")
        self.assertIn(f"→ touch {made}", ui.err.getvalue())
        self.assertIn("changes something", ui.err.getvalue())
        step = history.find("last").steps[0]
        self.assertEqual((step.kind, step.decision), ("proposal", "proposed"))

    def test_only_the_proposal_tool_is_offered(self):
        agent, provider, _, _ = self.make([[text("hi")]])
        tools = []
        original = provider.turn
        provider.turn = lambda system, t, *a, **k: (tools.append(t), original(system, t, *a, **k))[1]
        agent.ask("hello")
        self.assertEqual(tools, [[PROPOSE_TOOL]])
        self.assertIn("You do not run commands", agent.system)

    def test_unverified_read_is_marked(self):
        agent, _, ui, _ = self.make([[propose_use("frobnicate --all")]])
        agent.ask("x")
        self.assertIn("not verified as read-only", ui.err.getvalue())

    def test_continuing_sends_what_ran_redacted(self):
        agent, provider, _, history = self.make([
            [propose_use("df -h /")],
            [text("You have 16 GB free.")],
        ])
        agent.ask("how much space?")
        ran = Ran("df -h /", 0, f"/dev/disk3s1 460Gi 444Gi 16Gi key={SYNTHETIC_KEY}\n", True)
        entry = agent.ask("", ran)
        self.assertEqual(entry.outcome, "answered")
        sent = provider.requests[1][-1]["content"]
        self.assertEqual(sent[0]["tool_use_id"], "p1")
        self.assertIn("The user ran the proposed command.\nExit status 0.", sent[0]["content"])
        self.assertIn("16Gi", sent[0]["content"])
        self.assertNotIn(SYNTHETIC_KEY, json.dumps(provider.requests))
        self.assertEqual(len(sent), 1)  # no empty text block for a bare //
        steps = history.find("last").steps
        self.assertEqual((steps[0].command, steps[0].decision, steps[0].exit_code),
                         ("df -h /", "user-ran", 0))
        assert_every_tool_use_answered(self, provider.requests[1])

    def test_edited_interactive_and_not_run(self):
        cases = [
            (Ran("df -k /", 1, "", True), ["edited command instead:\n$ df -k /",
                                           "Exit status 1.", "(no output)"]),
            (Ran("less log", 0, "", False), ["not recorded (interactive)"]),
            (None, ["did not run the proposed command"]),
        ]
        for ran, expected in cases:
            with self.subTest(ran=ran):
                agent, provider, _, _ = self.make([[propose_use("df -h /")], [text("ok")]])
                agent.ask("space?")
                agent.ask("", ran)
                result = provider.requests[1][-1]["content"][0]["content"]
                for part in expected:
                    self.assertIn(part, result)

    def test_untracked_says_ush_cannot_tell(self):
        agent, provider, _, _ = self.make([[propose_use("df -h /")], [text("ok")]])
        agent.ask("space?")
        agent.ask("what now?", None, tracked=False)
        content = provider.requests[1][-1]["content"]
        self.assertIn("cannot tell whether the user ran", content[0]["content"])
        self.assertEqual(content[-1], {"type": "text", "text": "what now?"})

    def test_one_proposal_at_a_time_and_every_use_answered(self):
        agent, provider, ui, _ = self.make([
            [propose_use("df -h /", ident="p1"), propose_use("du -sh ~", ident="p2")],
            [text("done")],
        ])
        agent.ask("space?")
        self.assertEqual(agent.proposal["id"], "p1")
        self.assertNotIn("du -sh", ui.err.getvalue())
        agent.ask("", Ran("df -h /", 0, "x", True))
        results = provider.requests[1][-1]["content"]
        self.assertEqual([r["tool_use_id"] for r in results], ["p1", "p2"])
        self.assertIn("one command at a time", results[1]["content"])
        assert_every_tool_use_answered(self, provider.requests[1])

    def test_control_characters_are_never_proposed(self):
        hidden = "echo safe\x1b[2K\rrm -rf ~"
        agent, provider, ui, _ = self.make([
            [propose_use(hidden, ident="bad")],
            [propose_use("echo safe", ident="good")],
        ])
        entry = agent.ask("x")
        self.assertEqual(entry.outcome, "proposed")
        self.assertEqual(agent.proposal["command"], "echo safe")
        self.assertIn("control characters", json.dumps(provider.requests[1]))
        self.assertNotIn("rm -rf", ui.err.getvalue())
        for bidi in ("ls ‮", "ls ​"):
            agent, _, _, _ = self.make([[propose_use(bidi)], [text("no")]])
            self.assertEqual(agent.ask("x").outcome, "answered")

    def test_state_round_trip_continues_in_a_new_process(self):
        agent, _, _, _ = self.make([[propose_use("df -h /")]])
        agent.ask("space?")
        state = json.loads(json.dumps(agent.state()))
        again, provider, _, _ = self.make([[text("ok")]])
        again.restore(state)
        again.ask("", Ran("df -h /", 0, "16Gi", True))
        messages = provider.requests[0]
        self.assertEqual(messages[0]["content"][0]["text"], "space?")
        self.assertIn("16Gi", messages[-1]["content"][0]["content"])
        assert_every_tool_use_answered(self, messages)

    def test_restore_ignores_unknown_state(self):
        agent, _, _, _ = self.make([])
        for bad in ({}, {"version": 2, "messages": []}, {"version": 1, "messages": "x"}):
            agent.restore(bad)
            self.assertEqual((agent.messages, agent.proposal), ([], None))


class ExplanationsTest(TempDirTest):
    """Kept explanations (TODO 2.24): by the SHA-256 of the command line, the newest
    EXPLANATIONS_KEPT, in the same file as ushgo's."""

    def test_kept_by_hash_newest_first_out(self):
        kept = Explanations(os.path.join(self.tmp, "state"))
        self.assertIsNone(kept.get("echo hi"))
        kept.put("echo hi", "says hi")
        self.assertEqual(kept.get("  echo hi "), "says hi")
        with open(kept.path) as f:
            data = json.load(f)
        self.assertEqual(list(data), ["56a79f3b115448072387c2480044bfa2cf8f90e4f5fddd8c943b4e051b81f80b"])  # ushgo's key too
        self.assertEqual(os.stat(kept.path).st_mode & 0o777, 0o600)
        for i in range(EXPLANATIONS_KEPT):
            kept.put(f"echo {i}", f"says {i}")
        self.assertIsNone(kept.get("echo hi"))  # the oldest went
        self.assertEqual(kept.get("echo 0"), "says 0")
        kept.put("echo 0", "again")  # a new answer moves it to the newest
        kept.put("echo new", "new")
        self.assertEqual(kept.get("echo 0"), "again")
        self.assertIsNone(kept.get("echo 1"))
        with open(kept.path) as f:
            self.assertEqual(len(json.load(f)), EXPLANATIONS_KEPT)

    def test_a_broken_or_linked_store_is_ignored(self):
        state = os.path.join(self.tmp, "state")
        os.mkdir(state)
        kept = Explanations(state)
        with open(kept.path, "w") as f:
            f.write("not json")
        self.assertIsNone(kept.get("echo hi"))
        kept.put("echo hi", "says hi")
        self.assertEqual(kept.get("echo hi"), "says hi")
        os.remove(kept.path)
        target = os.path.join(self.tmp, "elsewhere.json")
        with open(target, "w") as f:
            json.dump({Explanations.key("echo hi"): {"answer": "planted"}}, f)
        os.symlink(target, kept.path)
        self.assertIsNone(kept.get("echo hi"))  # not read through a link


class HandoffTest(TempDirTest):
    def setUp(self):
        super().setUp()
        self.dir = os.path.join(self.tmp, "ush.abc123")
        os.mkdir(self.dir, 0o700)
        self.ident = fake_identity(self.tmp)

    def test_only_a_private_directory_of_the_user_is_used(self):
        env = {"USH_HANDOFF": self.dir}
        self.assertEqual(handoff_dir(self.ident, env), self.dir)
        self.assertIsNone(handoff_dir(fake_identity(self.tmp, uid=0), env))  # never as root
        self.assertIsNone(handoff_dir(self.ident, {"USH_HANDOFF": "relative"}))
        self.assertIsNone(handoff_dir(self.ident, {}))
        link = os.path.join(self.tmp, "link")
        os.symlink(self.dir, link)
        self.assertIsNone(handoff_dir(self.ident, {"USH_HANDOFF": link}))
        os.chmod(self.dir, 0o755)
        self.assertIsNone(handoff_dir(self.ident, env))
        os.chmod(self.dir, 0o700)
        self.assertIsNone(handoff_dir(fake_identity(self.tmp, uid=os.getuid() + 1), env))

    def test_proposal_replaces_earlier_records(self):
        for name in ("ran", "output"):
            with open(os.path.join(self.dir, name), "w") as f:
                f.write("old")
        write_proposal(self.dir, "df -h /", interactive=False)
        self.assertEqual(sorted(os.listdir(self.dir)), ["proposal"])
        with open(os.path.join(self.dir, "proposal")) as f:
            self.assertEqual(f.read(), "capture\ndf -h /")
        self.assertEqual(os.stat(os.path.join(self.dir, "proposal")).st_mode & 0o777, 0o600)
        write_proposal(self.dir, "less log", interactive=True)
        with open(os.path.join(self.dir, "proposal")) as f:
            self.assertEqual(f.read(), "interactive\nless log")

    def test_take_ran_reads_bounds_and_removes(self):
        self.assertIsNone(take_ran(self.dir, 1000))
        with open(os.path.join(self.dir, "ran"), "w") as f:
            f.write("3\ngrep -r x .\n")
        with open(os.path.join(self.dir, "output"), "w") as f:
            f.write("a" * 5000 + "END")
        ran = take_ran(self.dir, 1000)
        self.assertEqual((ran.command, ran.exit_code, ran.captured, ran.truncated),
                         ("grep -r x .", 3, True, True))
        self.assertLessEqual(len(ran.output), 1100)
        self.assertTrue(ran.output.endswith("END"))
        self.assertEqual(os.listdir(self.dir), [])
        with open(os.path.join(self.dir, "ran"), "w") as f:
            f.write("x\nvim notes")
        ran = take_ran(self.dir, 1000)
        self.assertEqual((ran.exit_code, ran.captured), (None, False))

    def test_take_ran_refuses_a_symlinked_output(self):
        secret = os.path.join(self.tmp, "secret")
        with open(secret, "w") as f:
            f.write("TOP SECRET")
        with open(os.path.join(self.dir, "ran"), "w") as f:
            f.write("0\ncat x")
        os.symlink(secret, os.path.join(self.dir, "output"))
        ran = take_ran(self.dir, 1000)
        self.assertNotIn("TOP SECRET", ran.output)
        self.assertFalse(ran.captured)

    def test_sessions_are_private_per_shell_and_expire(self):
        state = os.path.join(self.tmp, "state")
        self.assertEqual(session_key(self.dir), "ush.abc123")
        self.assertEqual(session_key(None), f"ppid-{os.getppid()}")
        mine, other = Sessions(state, "ush.abc123"), Sessions(state, "ush.other")
        mine.save({"version": 1, "messages": []})
        self.assertEqual(os.stat(mine.path).st_mode & 0o777, 0o600)
        self.assertEqual(os.stat(mine.directory).st_mode & 0o777, 0o700)
        self.assertEqual(mine.load(), {"version": 1, "messages": []})
        self.assertIsNone(other.load())
        old = time.time() - SESSION_TTL - 10
        os.utime(mine.path, (old, old))
        self.assertIsNone(mine.load())
        self.assertFalse(os.path.exists(mine.path))
        other.save({"a": 1})
        os.utime(other.path, (old, old))
        mine.save({"b": 2})  # saving prunes other shells' expired conversations
        self.assertFalse(os.path.exists(other.path))
        mine.clear()
        self.assertIsNone(mine.load())
        for bad in ("../x", "", "a/b", "x" * 65):
            with self.assertRaises(ValueError):
                Sessions(state, bad)
