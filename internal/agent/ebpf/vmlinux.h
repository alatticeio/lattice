/*
 * Minimal vmlinux.h for lattice eBPF TC ingress program.
 * Contains only the kernel types required for L2/L3/L4 packet parsing.
 *
 * Replace with a full bpftool-generated vmlinux.h for production:
 *   bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h
 */

#ifndef __VMLINUX_H__
#define __VMLINUX_H__

typedef unsigned char __u8;
typedef signed char __s8;
typedef unsigned short __u16;
typedef signed short __s16;
typedef unsigned int __u32;
typedef signed int __s32;
typedef unsigned long long __u64;
typedef signed long long __s64;

/* Big-endian types */
typedef __u16 __be16;
typedef __u32 __be32;
typedef __u64 __be64;

/* Checksum type */
typedef __u32 __wsum;

typedef unsigned char u8;
typedef unsigned short u16;
typedef unsigned int u32;
typedef unsigned long long u64;

struct __sk_buff {
    __u32 len;
    __u32 pkt_type;
    __u32 mark;
    __u32 queue_mapping;
    __u32 cookie;
    __u32 priority;
    __u32 ifindex;
    __u32 tx_queue_mapping;
    __s32 hash;
    __u32 tc_index;
    __u32 cb[5];
    __u32 hwtstamp;
    __u32 route_optmask;
    __s32 tc_classid;
    __u16 data_meta;
    union {
        __u8 *data;
        __u64 data_;
    };
    union {
        __u8 *data_end;
        __u64 data_end_;
    };
};

struct ethhdr {
    unsigned char h_dest[6];
    unsigned char h_source[6];
    __u16 h_proto;
};

struct iphdr {
    __u8 version:4, ihl:4;
    __u8 tos;
    __u16 tot_len;
    __u16 id;
    __u16 frag_off;
    __u8 ttl;
    __u8 protocol;
    __u16 check;
    __u32 saddr;
    __u32 daddr;
};

struct tcphdr {
    __u16 source;
    __u16 dest;
    __u32 seq;
    __u32 ack_seq;
    __u16 res1:4, doff:4, fin:1, syn:1, rst:1, psh:1, ack:1, urg:1, ece:1, cwr:1;
    __u16 window;
    __u16 check;
    __u16 urg_ptr;
};

#define IPPROTO_TCP 6
#define IPPROTO_UDP 17

/* BPF map types */
enum bpf_map_type {
    BPF_MAP_TYPE_UNSPEC,
    BPF_MAP_TYPE_HASH,
    BPF_MAP_TYPE_ARRAY,
    BPF_MAP_TYPE_PROG_ARRAY,
    BPF_MAP_TYPE_PERF_EVENT_ARRAY,
    BPF_MAP_TYPE_PERCPU_HASH,
    BPF_MAP_TYPE_PERCPU_ARRAY,
    BPF_MAP_TYPE_STACK_TRACE,
    BPF_MAP_TYPE_CGROUP_ARRAY,
    BPF_MAP_TYPE_LRU_HASH,
    BPF_MAP_TYPE_LRU_PERCPU_HASH,
    BPF_MAP_TYPE_LPM_TRIE,
    BPF_MAP_TYPE_ARRAY_OF_MAPS,
    BPF_MAP_TYPE_HASH_OF_MAPS,
    BPF_MAP_TYPE_DEVMAP,
    BPF_MAP_TYPE_SOCKMAP,
    BPF_MAP_TYPE_CPUMAP,
    BPF_MAP_TYPE_XSKMAP,
    BPF_MAP_TYPE_SOCKHASH,
    BPF_MAP_TYPE_CGROUP_STORAGE,
    BPF_MAP_TYPE_REUSEPORT_SOCKARRAY,
    BPF_MAP_TYPE_PERCPU_CGROUP_STORAGE,
    BPF_MAP_TYPE_QUEUE,
    BPF_MAP_TYPE_STACK,
    BPF_MAP_TYPE_SK_STORAGE,
    BPF_MAP_TYPE_DEVMAP_HASH,
    BPF_MAP_TYPE_STRUCT_OPS,
    BPF_MAP_TYPE_RINGBUF,
    BPF_MAP_TYPE_INODE_STORAGE,
    BPF_MAP_TYPE_TASK_STORAGE,
    BPF_MAP_TYPE_BLOOM_FILTER,
};

/* BPF map flags */
#define BPF_F_NO_PREALLOC (1U << 0)
#define BPF_F_NO_COMMON_LRU (1U << 1)
#define BPF_F_NUMA_NODE (1U << 2)
#define BPF_F_RDONLY (1U << 3)
#define BPF_F_WRONLY (1U << 4)
#define BPF_F_STACK_BUILD_ID (1U << 5)
#define BPF_F_ZERO_SEED (1U << 6)

#endif /* __VMLINUX_H__ */
