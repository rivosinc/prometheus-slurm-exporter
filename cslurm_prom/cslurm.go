package cslurm_prom

// #include <slurm/slurm.h>
// #include <stdlib.h>
import "C"

import (
	"os"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

type SlurmVersion struct {
	Major int
	Micro int
	Minor int
}

func init() {
	slurm_conf, ok := os.LookupEnv("SLURM_CONF")
	if !ok {
		slurm_conf = "/etc/slurm/slurm.conf"
	}
	C.slurm_init(C.CString(slurm_conf))
}

func CGetPartitions() int {
	size := unsafe.Sizeof(C.struct_partition_info_msg_t{})
	partitions := (*C.struct_partition_info_msg_t)(C.malloc(C.size_t(size)))
	ret := C.slurm_load_partitions(0, (**C.struct_partition_info_msg)(unsafe.Pointer(&partitions)), 0x0001)
	return int(ret)
}

// tests work around due to the fact that we can't use the C and testing packages together. Thus we wrap them here
func testCGetPartitions(t *testing.T) {
	assert := assert.New(t)
	assert.Zero(CGetPartitions())
}
