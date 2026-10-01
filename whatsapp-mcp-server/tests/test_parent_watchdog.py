import json
import os
import signal
import subprocess
import sys
import time
from pathlib import Path

SERVER_DIR = Path(__file__).resolve().parent.parent

# Stands in for `uv run main.py`: starts the server with a stdin pipe it keeps
# open, reports its own pid and the server's, then waits to be killed. Its own
# pid is printed because on Windows sys.executable may be the venv launcher,
# and killing the launcher would leave this interpreter running.
LAUNCHER = """
import os, subprocess, sys, time
server = subprocess.Popen([sys.executable, "main.py"], stdin=subprocess.PIPE,
                          stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
print(os.getpid(), server.pid, flush=True)
time.sleep(120)
"""


def _alive(pid: int) -> bool:
    if sys.platform == "win32":
        out = subprocess.run(["tasklist", "/FI", f"PID eq {pid}", "/NH"],
                             capture_output=True, text=True).stdout
        return str(pid) in out.split()
    try:
        os.kill(pid, 0)
        return True
    except OSError:
        return False


def _kill(pid: int) -> None:
    if sys.platform == "win32":
        subprocess.run(["taskkill", "/F", "/PID", str(pid)], capture_output=True)
    else:
        os.kill(pid, signal.SIGKILL)


def test_server_exits_when_its_launcher_is_killed():
    launcher = subprocess.Popen([sys.executable, "-c", LAUNCHER], cwd=SERVER_DIR,
                                stdout=subprocess.PIPE, text=True)
    launcher_pid, server_pid = map(int, launcher.stdout.readline().split())
    try:
        time.sleep(3)  # let the server import and start serving
        assert _alive(server_pid)

        _kill(launcher_pid)

        deadline = time.time() + 10
        while _alive(server_pid) and time.time() < deadline:
            time.sleep(0.2)
        assert not _alive(server_pid)
    finally:
        for pid in (server_pid, launcher_pid):
            if _alive(pid):
                _kill(pid)
        launcher.kill()
        launcher.wait()


def test_server_still_answers_while_its_launcher_lives():
    server = subprocess.Popen([sys.executable, "main.py"], cwd=SERVER_DIR,
                              stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                              stderr=subprocess.DEVNULL, text=True)
    try:
        request = {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {
            "protocolVersion": "2024-11-05", "capabilities": {},
            "clientInfo": {"name": "test", "version": "0"}}}
        server.stdin.write(json.dumps(request) + "\n")
        server.stdin.flush()
        reply = json.loads(server.stdout.readline())
        assert reply["result"]["serverInfo"]["name"] == "whatsapp"
    finally:
        server.kill()
        server.wait()
