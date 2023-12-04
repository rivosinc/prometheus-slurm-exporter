#include <stdlib.h>
#include <slurm/slurm.h>
#include <iostream>

int main() {
    partition_info_msg_t *p_info = (partition_info_msg_t *) malloc(sizeof(partition_info_msg_t));
    int err = slurm_load_partitions(0, &p_info, SHOW_ALL);
    printf("%d\n", err);
    return 0;
}
