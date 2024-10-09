#define _GNU_SOURCE
#include <sched.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/stat.h>
#include <string.h>
#include <fcntl.h>

extern char **environ;


int set_sysctl(const char *path, const char *value) {
    int fd = open(path, O_WRONLY);

    if (fd == -1) {
        perror("open");
        return -1;
    }

    if (write(fd, value, strlen(value)) == -1) {
        perror("write");
        return -1;
    }

    close(fd);

    return 0;
}

int main(int argc, char **argv) {
	int fdm = open("/proc/1/ns/mnt", O_RDONLY);
	int fdu = open("/proc/1/ns/uts", O_RDONLY);
	int fdn = open("/proc/1/ns/net", O_RDONLY);
	int fdi = open("/proc/1/ns/ipc", O_RDONLY);
	int froot = open("/proc/1/root", O_RDONLY);

	if (fdm == -1 || fdu == -1 || fdn == -1 || fdi == -1 || froot == -1) {
		fprintf(stderr, "Failed to open /proc/1 files, are you root?\n");
		exit(1);
	}

	if (setns(fdm, 0) == -1) {
		perror("setns:mnt");
		exit(1);
	}
	if (setns(fdu, 0) == -1) {
		perror("setns:uts");
		exit(1);
	}
	if (setns(fdn, 0) == -1) {
		perror("setns:net");
		exit(1);
	}
	if (setns(fdi, 0) == -1) {
		perror("setns:ipc");
		exit(1);
	}
	if (fchdir(froot) == -1) {
		perror("fchdir");
		exit(1);
	}
	if (chroot(".") == -1) {
		perror("chroot");
		exit(1);
	}

	if (set_sysctl("/proc/sys/net/core/rmem_max", "7500000") == -1) {
        perror("set_sysctl for rmem_max failed");
        exit(1);
    }

    if (set_sysctl("/proc/sys/net/core/wmem_max", "7500000") == -1) {
        perror("set_sysctl for wmem_max failed");
        exit(1);
    }

    if (set_sysctl("/proc/sys/vm/overcommit_memory", "1") == -1) {
        perror("set_sysctl for overcommit_memory failed");
        exit(1);
    }

    if (set_sysctl("/sys/kernel/mm/transparent_hugepage/enabled", "never") == -1) {
        perror("transparent_hugepage failed");
        exit(1);
    }

    struct stat st = {0};

    if (stat("/etc/sysctl.d", &st) == -1) {
        perror("/etc/sysctl.d is not existing skipping");
        exit(0);
    }

    FILE *fp = fopen("/etc/sysctl.d/99-tanjun.conf", "w");

    if (fp == NULL) {
        perror("fopen /etc/sysctl.d/99-tanjun.conf");
        exit(1);
    }

    fprintf(fp, "net.core.rmem_max=7500000\n");
    fprintf(fp, "net.core.wmem_max=7500000\n");
    fprintf(fp, "vm.overcommit_memory=1\n");

    fclose(fp);

    for (;;) pause();

	exit(0);
}
