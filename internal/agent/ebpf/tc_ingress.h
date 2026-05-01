#ifndef __TC_INGRESS_H
#define __TC_INGRESS_H

#define ETH_P_IP  0x0800
#define TC_ACT_OK 0
#define TC_ACT_SHOT 2

#define MAX_POLICY_ENTRIES 4096

struct ip_key {
    u32 lpm_key;
    u32 src_ip;
};

struct port_key {
    u32 lpm_key;
    u32 src_ip;
    u8 protocol;
    u16 dst_port;
    u8 padding;
};

#endif /* __TC_INGRESS_H */
