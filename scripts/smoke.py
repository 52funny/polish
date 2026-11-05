#!/usr/bin/env python3
"""Native Wayland smoke test using a local mock model and isolated settings.

Build: go build -tags e2e -o bin/polish-e2e ./cmd/polish
Run:   python scripts/smoke.py
No real model credentials, clipboard writes, or desktop config changes.
"""

import base64
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class Model(BaseHTTPRequestHandler):
    requests = []
    models_available = True

    def log_message(self, *args):
        pass

    def do_GET(self):
        assert self.path == "/v1/models"
        assert self.headers["Authorization"] == "Bearer smoke-key"
        data = json.dumps({"data": [{"id": "another-model"}, {"id": "smoke-model"}]}).encode()
        self.send_response(200 if self.models_available else 404)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        assert self.path == "/v1/chat/completions"
        assert self.headers["Authorization"] == "Bearer smoke-key"
        self.requests.append(body)
        output = "Polished text." if "Polish the source text" in body["messages"][0]["content"] else "Translated text."
        data = json.dumps({"choices": [{"message": {"content": output}, "finish_reason": "stop"}]}).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)


def main():
    binary = str(Path("bin/polish-e2e").resolve())
    with tempfile.TemporaryDirectory(prefix="polish-smoke-") as tmp:
        root = Path(tmp)
        mockbin = root / "mockbin"
        mockbin.mkdir()
        (root / "selection").write_text("原始文本。\n第二段。")
        # Mock only the two clipboard helpers; the GUI and single-instance IPC are real.
        for name, script in {
            "wl-paste": 'from pathlib import Path\nimport os,sys\np=Path(os.environ["POLISH_SMOKE_DIR"])/"selection"\nif not p.exists(): sys.exit(1)\nsys.stdout.write(p.read_text())\n',
            "wl-copy": 'from pathlib import Path\nimport os,sys\n(Path(os.environ["POLISH_SMOKE_DIR"])/"copied").write_text(sys.stdin.read())\n',
        }.items():
            p = mockbin / name
            p.write_text("#!/usr/bin/env python3\n" + script)
            p.chmod(0o700)
        server = ThreadingHTTPServer(("127.0.0.1", 0), Model)
        threading.Thread(target=server.serve_forever, daemon=True).start()
        env = dict(os.environ, XDG_CONFIG_HOME=str(root / "config"),
                   MYGO_AUTOMATION=str(root / "app.sock"), POLISH_SMOKE_DIR=tmp,
                   PATH=str(mockbin) + os.pathsep + os.environ["PATH"],
                   XDG_DATA_HOME=str(root / "data"), LANGUAGE="en", LC_ALL="C.UTF-8")
        secondary_env = {k: v for k, v in env.items() if k != "MYGO_AUTOMATION"}
        # Launch outside the source tree to exercise embedded resources.
        proc = subprocess.Popen([binary, "--selection"], env=env, cwd=root, stdout=subprocess.DEVNULL)
        conn = socket.socket(socket.AF_UNIX)
        conn.settimeout(5)
        try:
            for _ in range(100):
                if proc.poll() is not None:
                    raise RuntimeError(f"App exited before ready: {proc.returncode}")
                try:
                    conn.connect(env["MYGO_AUTOMATION"])
                    break
                except (FileNotFoundError, ConnectionRefusedError):
                    time.sleep(0.05)
            else:
                raise RuntimeError("Automation socket did not start")
            stream = conn.makefile("rwb")

            def call(method, **params):
                stream.write((json.dumps({"id": 1, "method": method, "params": params}) + "\n").encode())
                stream.flush()
                response = json.loads(stream.readline())
                if method == "tree" and response.get("error", {}).get("code") == "no_window":
                    return {"nodes": []}  # App ready, first window not created yet.
                assert "error" not in response, response
                return response["result"]

            def nodes():
                return call("tree")["nodes"]

            def wait_for(predicate):
                for _ in range(100):
                    tree = nodes()
                    if predicate(tree):
                        return tree
                    time.sleep(0.05)
                raise AssertionError("Expected UI state did not appear: " + str([(n.get("role"), n.get("label"), n.get("actions")) for n in tree]))

            def value(key, expected):
                return lambda tree: any(n.get("key") == key and n.get("value", "") == expected for n in tree)

            def press(key):
                # A prior action may have switched views before its next frame.
                wait_for(lambda tree: any(n.get("key") == key for n in tree))
                call("act", key=key, action="press")

            wait_for(lambda tree: any(n.get("key") == "base-url" for n in tree))
            def choose_option(key, label):
                press(key)
                tree = wait_for(lambda tree: any(n.get("label") == label and n.get("role") == "text" for n in tree))
                choice = next(n for n in tree if n.get("label") == label and n.get("role") == "text")
                b = choice["bounds"]
                call("input", events=[{"type": "click", "x": b["x"] + b["width"] / 2, "y": b["y"] + b["height"] / 2}])
                call("idle")

            choose_option("language", "简体中文")
            wait_for(lambda tree: any(n.get("label") == "连接大模型" for n in tree))
            press("save-settings")
            wait_for(lambda tree: any("请填写有效的 HTTP(S)" in n.get("label", "") for n in tree))
            screenshot = call("capture")
            output = Path(os.environ.get("POLISH_TEST_ARTIFACTS", "/tmp/polish-preview"))
            output.mkdir(parents=True, exist_ok=True)
            (output / "settings-zh-Hans.png").write_bytes(base64.b64decode(screenshot["png"]))
            choose_option("language", "English")
            wait_for(lambda tree: any("Enter a valid HTTP(S)" in n.get("label", "") for n in tree))
            assert json.loads((root / "config/polish/config.json").read_text())["language"] == "en"
            for key, text in {"base-url": f"http://127.0.0.1:{server.server_port}/v1", "api-key": "smoke-key"}.items():
                call("act", key=key, action="setValue", text=text)
                call("idle")
            press("fetch-models")
            wait_for(lambda tree: any(n.get("key") == "model-select" for n in tree))
            press("model-select")
            tree = wait_for(lambda tree: any(n.get("label") == "smoke-model" for n in tree))
            choice = next(n for n in tree if n.get("label") == "smoke-model")
            bounds = choice["bounds"]
            call("input", events=[{"type": "click", "x": bounds["x"] + bounds["width"] / 2,
                                    "y": bounds["y"] + bounds["height"] / 2}])
            call("idle")
            screenshot = call("capture")
            output = Path(os.environ.get("POLISH_TEST_ARTIFACTS", "/tmp/polish-preview"))
            output.mkdir(parents=True, exist_ok=True)
            (output / "model-picker-native.png").write_bytes(base64.b64decode(screenshot["png"]))
            press("save-settings")
            wait_for(value("source", "原始文本。\n第二段。"))
            settings = root / "config/polish/config.json"
            assert settings.stat().st_mode & 0o777 == 0o600
            assert json.loads(settings.read_text())["model"] == "smoke-model"
            press("replace-newlines")
            wait_for(value("source", "原始文本。 第二段。"))
            assert json.loads(settings.read_text())["replace_newlines"] is True
            press("process")
            wait_for(value("result", "Polished text."))
            press("copy")
            wait_for(lambda tree: any(n.get("label") == "Result copied." for n in tree))
            assert (root / "copied").read_text() == "Polished text."
            tab = next(n for n in nodes() if n.get("label") == "Translate" and "press" in n.get("actions", []))
            call("act", id=tab["id"], action="press")
            wait_for(lambda tree: any(n.get("key") == "target" for n in tree))
            press("target")
            tree = wait_for(lambda tree: any(n.get("label") == "English" for n in tree))
            english = next(n for n in tree if n.get("label") == "English")
            bounds = english["bounds"]
            call("input", events=[{"type": "click", "x": bounds["x"] + bounds["width"] / 2,
                                    "y": bounds["y"] + bounds["height"] / 2}])
            call("idle")
            press("process")
            wait_for(value("result", "Translated text."))
            assert len(Model.requests) == 2
            assert "into English" in Model.requests[1]["messages"][0]["content"]
            assert all(r["messages"][1]["content"] == "原始文本。 第二段。" for r in Model.requests)
            # Discovery failures preserve the saved model for manual configuration.
            press("settings-toggle")
            Model.models_available = False
            press("fetch-models")
            wait_for(lambda tree: any("does not provide a model list" in n.get("label", "") for n in tree))
            assert value("model", "smoke-model")(nodes())
            press("settings-toggle")
            press("replace-newlines")
            (root / "selection").write_text("保留换行\n原文")
            press("paste")
            wait_for(value("source", "保留换行\n原文"))
            assert json.loads(settings.read_text())["replace_newlines"] is False
            # One existing native window is reused; the new selection wins.
            (root / "selection").write_text("新的选区")
            subprocess.run([binary, "--selection"], env=secondary_env, cwd=root, check=True, timeout=5)
            wait_for(value("source", "新的选区"))
            assert len(call("windows")["windows"]) == 1
            assert value("result", "")(nodes())
            (root / "selection").unlink()
            subprocess.run([binary, "--selection"], env=secondary_env, cwd=root, check=True, timeout=5)
            wait_for(lambda tree: value("source", "")(tree) and any("Could not read the selection" in n.get("label", "") for n in tree))
            # Capture only this application's content, without desktop pixels.
            screenshot = call("capture")
            output = Path(os.environ.get("POLISH_TEST_ARTIFACTS", "/tmp/polish-preview"))
            output.mkdir(parents=True, exist_ok=True)
            (output / "native-smoke.png").write_bytes(base64.b64decode(screenshot["png"]))
            print("PASS: live language switching, localized errors, model discovery, dropdown selection and persistence, manual fallback, model polish/translation (mock), copy helper, selection failure, single-instance reuse")
            print(f"Preview: {output / 'native-smoke.png'}")
        finally:
            conn.close()
            if proc.poll() is None:
                proc.terminate()
                proc.wait(timeout=5)
            server.shutdown()


if __name__ == "__main__":
    main()
