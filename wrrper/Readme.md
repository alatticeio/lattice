## WRRP 

Offset (Bytes)  0            1            2            3
0              |     'W'    |     'R'    |     'R'    |     'P'    |  (Magic Number)
4              |    Version (High)   | Version (Low) |    Cmd    |  Reserved |
8              |                  Payload Length (4 Bytes)           |
12             |                                                     |
16             |                                                     |
20             |                                                     |
24             |              Session ID (28 Bytes)                  |
28             |                                                     |
32             |                                                     |
36             |                                                     |
40             |                    Payload Data ...                 |