# xreview Installation for Codex CLI

Follow these steps to install xreview.

## Step 1: Install xreview binary

Run:
```bash
curl -fsSL https://raw.githubusercontent.com/davidleitw/xreview/master/scripts/install.sh | bash
```

If this fails, tell the user to download manually from https://github.com/davidleitw/xreview/releases

## Step 2: Install xreview skill

Run:
```bash
mkdir -p ~/.agents/skills/xreview
curl -fsSL -o ~/.agents/skills/xreview/SKILL.md https://raw.githubusercontent.com/davidleitw/xreview/master/.agents/skills/xreview/SKILL.md
```

## Step 3: Verify

Run: `xreview version`

If the command succeeds, tell the user:
- xreview is installed and ready
- Restart Codex to pick up the new skill
- They can then ask "review my code" to start a review

If it fails, check that `~/.local/bin` is in PATH and suggest adding it.
