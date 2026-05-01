//go:build ignore

// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

#include "vmlinux.h"
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_tracing.h>

#include "tc_ingress.h"

char __license[] SEC("license") = "Apache-2.0";

struct {
    __uint(type, BPF_MAP_TYPE_LPM_TRIE);
    __type(key, struct ip_key);
    __type(value, u8);
    __uint(max_entries, MAX_POLICY_ENTRIES);
    __uint(map_flags, BPF_F_NO_PREALLOC);
} ingress_policy_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __type(key, struct port_key);
    __type(value, u8);
    __uint(max_entries, MAX_POLICY_ENTRIES);
} ingress_port_policy_map SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __type(key, u32);
    __type(value, u8);
    __uint(max_entries, 1);
} default_action_map SEC(".maps");

static __always_inline u8 get_default_action(void) {
    u32 idx = 0;
    u8 *val = bpf_map_lookup_elem(&default_action_map, &idx);
    return val ? *val : 0;
}

SEC("tc")
int lattice_tc_ingress(struct __sk_buff *skb) {
    void *data = (void *)(long)skb->data;
    void *data_end = (void *)(long)skb->data_end;

    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end)
        return TC_ACT_OK;

    u16 h_proto = bpf_ntohs(eth->h_proto);
    if (h_proto != ETH_P_IP)
        return TC_ACT_OK;

    data += sizeof(*eth);

    struct iphdr *iph = data;
    if ((void *)(iph + 1) > data_end)
        return TC_ACT_OK;

    u32 src_ip = iph->saddr;

    struct port_key pkey = {};
    pkey.lpm_key = 32;
    pkey.src_ip = src_ip;
    pkey.protocol = iph->protocol;

    u32 ip_hdr_len = iph->ihl * 4;
    if (iph->protocol == IPPROTO_TCP || iph->protocol == IPPROTO_UDP) {
        void *l4 = data + ip_hdr_len;
        if ((void *)l4 + sizeof(struct tcphdr) <= data_end) {
            struct tcphdr *tcph = l4;
            pkey.dst_port = bpf_ntohs(tcph->dest);
        }
    }

    u8 *action = bpf_map_lookup_elem(&ingress_port_policy_map, &pkey);
    if (action) {
        return *action ? TC_ACT_OK : TC_ACT_SHOT;
    }

    struct ip_key ikey = {};
    ikey.lpm_key = 32;
    ikey.src_ip = src_ip;

    action = bpf_map_lookup_elem(&ingress_policy_map, &ikey);
    if (action) {
        return *action ? TC_ACT_OK : TC_ACT_SHOT;
    }

    if (get_default_action())
        return TC_ACT_OK;
    return TC_ACT_SHOT;
}
