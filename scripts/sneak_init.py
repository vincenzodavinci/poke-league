#!/usr/bin/env python3
"""Arbok's other trick: hide the poisoned image in an init container.

  sneak_init.py <arbok.yaml> <signed image>

Prints Arbok's manifest with the app container switched to a properly signed
image and the poisoned one moved into an init container (hardened and sized, so
every other gym passes). Koga has to check init containers to catch it. Used
only for server-side dry runs (`make check`). Stdlib only: the manifest is
edited as text, and every edit must match exactly once.
"""
import re
import sys


def main():
    path, signed = sys.argv[1], sys.argv[2]
    with open(path) as f:
        text = f.read()
    m = re.search(r"^          image: (\S+)$", text, re.M)
    if not m or len(re.findall(r"^          image: ", text, re.M)) != 1:
        sys.exit(f"{path}: expected exactly one container image")
    poisoned = m.group(1)
    text = text.replace(m.group(0), f"          image: {signed}")
    init = f"""      initContainers:
        - name: warm-up
          image: {poisoned}
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: {{drop: ["ALL"]}}
          resources:
            requests: {{cpu: 10m, memory: 32Mi}}
            limits: {{memory: 96Mi}}
      containers:
"""
    if text.count("      containers:\n") != 1:
        sys.exit(f"{path}: expected exactly one containers: list")
    text = text.replace("      containers:\n", init)
    sys.stdout.write(re.sub(r"\barbok\b(?!\.yaml)", "arbok-init", text))


if __name__ == "__main__":
    main()
