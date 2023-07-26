#!/usr/bin/python3

import psutil
import os
import requests
from time import sleep
import dataclasses
from typing import Generator
import argparse as ag
import json
import platform


# must correlate with trace info struct
@dataclasses.dataclass
class TraceInfo:
    pid: int
    cpus: float
    threads: float
    mem: float
    read_bytes: float
    write_bytes: float
    job_id: int
    username: str = os.getenv("USER", "")
    hostname: str = platform.node()


class ProcWrapper:
    """thin wrapper to send slurm proc metrics to our exporter"""

    sample_rate: int
    jobid: int
    proc: psutil.Popen

    def __init__(self, cmd=[], sample_rate=0, jobid=0):
        self.cmd = cmd
        self.sample_rate = sample_rate
        self.jobid = jobid
        assert self.jobid > 0, "SLURM_JOBID must be provided"
        assert self.cmd, "no cmd provided"
        assert self.sample_rate > 0, "endpoint must be greater than 0"
        self.proc = psutil.Popen(self.cmd)

    def poll_info(self) -> Generator[TraceInfo, None, None]:
        while self.proc.poll() is None:
            io_counters = self.proc.io_counters()
            yield TraceInfo(
                pid=self.proc.pid,
                cpus=self.proc.cpu_percent(),
                threads=self.proc.num_threads(),
                write_bytes=io_counters.write_bytes,
                read_bytes=io_counters.read_bytes,
                mem=self.proc.memory_info().rss,
                job_id=self.jobid,
            )
            sleep(self.sample_rate)


if __name__ == "__main__":
    parser = ag.ArgumentParser(
        "cmd wrapper",
        """
Simple wrapper on any proccess using proc utils can use it inline, exp.
    $ python proctrac.py sleep 10
the wrapper will then resolve sample rate for SAMPLE_RATE env var and
endpoint url for the slurm exporter from the SLURM_EXPORTER env var.
Or by passing explicit cmdline args, exp.
    $ python proctrac.py --endpoint localhost:8092 --sample-rate 10 --cmd sleep 10
""",
        "This script is intended to be called from within a sbatch script wrapper",
    )
    parser.add_argument("argv", nargs="*")
    parser.add_argument(
        "--endpoint",
        help="endpoint for slurm exporter",
        default=os.getenv("SLURM_EXPORTER", "localhost:8092"),
    )
    parser.add_argument(
        "--sample-rate",
        type=float,
        help="rate to sample wrapped proc",
        default=float(os.getenv("SAMPLE_RATE", 10)),
    )
    parser.add_argument("--cmd", nargs="+")
    parser.add_argument(
        "--jobid",
        type=int,
        help="explicitly passing slurm job id (very rarely needed)",
        default=int(os.getenv("SLURM_JOBID", 0)),
    )
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument(
        "--validate",
        action="store_true",
        help="run the poll once to check for schema correctness",
    )
    args = parser.parse_args()
    assert not (args.argv and args.cmd), "argv and --cmd are mutually exclusive"
    assert args.argv or args.cmd, "must provide an commnad to wrap"
    wrapper = ProcWrapper(args.cmd or args.argv, args.sample_rate, args.jobid)

    if args.validate:
        print(json.dumps(dataclasses.asdict(next(wrapper.poll_info()))))
        wrapper.proc.terminate()
    elif args.dry_run:
        [print(json.dumps(dataclasses.asdict(stat))) for stat in wrapper.poll_info()]
    else:
        [
            requests.post(args.endpoint, json=dataclasses.asdict(trace))
            for trace in wrapper.poll_info()
        ]
