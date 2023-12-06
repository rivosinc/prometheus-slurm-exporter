#include <stdlib.h>
#include <slurm/slurm.h>

static partition_info_msg_t *part_info_msg = NULL;

int main() {
    slurm_init(NULL);
    int err = slurm_load_partitions(0, &part_info_msg, SHOW_DETAIL);
    if (err != 0) return err;
    printf("num partitions %d\n", part_info_msg->record_count);
    if (part_info_msg != NULL) {
        slurm_free_partition_info_msg(part_info_msg);
        part_info_msg = NULL;
    }
    slurm_fini();
    return err;
}
