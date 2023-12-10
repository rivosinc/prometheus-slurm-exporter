package cslurm_prom

import "github.com/rivosinc/prometheus-slurm-exporter/cslurm_prom"

func main() {
	exp := cslurm_prom.NewMetricExporter()
	defer cslurm_prom.DeleteMetricExporter(exp)
	exp.CollectNodeInfo()
	exp.Print()
}
