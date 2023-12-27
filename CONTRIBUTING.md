<!--
SPDX-FileCopyrightText: 2023 Rivos Inc.

SPDX-License-Identifier: Apache-2.0
-->
# Developing the C extension

## Developing w/ Docker

```bash
# should take about 5-10 min
just docker
```

This should be all that's required to get started. This will launch a single node slurm cluster upon instantiation. If for some reason, these services are killed, run `./entrypoint.sh bash` within the container. This container is equiped with everything needed to contribute to the repo out of the box.

### Opening in VScode

Download the following extensions:
  - [Dev Container](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
  - [SSH](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-ssh)

If building the image natively fails, users can build docker with `--platform linux/amd64`. After building the container,
Open Vscode (`Cmd/Ctrl+Shift+P`) and run with the following:

![launch dev container](<images/dev_container_launch.png>)

This should pull our configured dev container. From there, our standard plugins should work with minimal modifications:

  - [Go](https://marketplace.visualstudio.com/items?itemName=golang.Go)
  - [Python](https://marketplace.visualstudio.com/items?itemName=ms-python.python)
  - [C/C++](https://marketplace.visualstudio.com/items?itemName=ms-vscode.cpptools)

For the C/C++ extension, add the following include path to `.vscode/c_cpp_properties.json`

```json
{
    "configurations": [
        {
            "name": "Linux",
            "includePath": [
                "${workspaceFolder}/**",
                "/usr/lib64/include"
            ],
            "defines": [],
            "compilerPath": "/usr/bin/gcc",
            "cStandard": "c17",
            "cppStandard": "gnu++14",
            "intelliSenseMode": "linux-gcc-x64"
        }
    ],
    "version": 4
}
```

### Developing Locally
Download slurm and associated headers. This will typically involve [downloading](https://github.com/SchedMD/slurm/tags) a slurm release and
configuring and installing the repo. Note, installing the RPM/apt packages won't install the headers that the extension needs.
After installation, modify the variables in your `.env` file and invoke via the `justfile`

| Variable          | Default Value        | Purpose                                                                     |
|-------------------|----------------------|-----------------------------------------------------------------------------|
| SLURM_LIB_DIR     | /usr/lib64/lib/slurm | directory where `libslurm.so` is located                                    |
| SLURM_INCLUDE_DIR | /usr/lib64/include   | location of `slurm/slurm.h`                                                 |