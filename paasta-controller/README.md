# PaaS-TA Controller Release

## Deploy

### paasta-controller 설치하는 절차는 아래와 같습니다.

### 1. bosh create release --force --name paasta-controller --version 2.0
### 2. bosh upload release

### bosh releases 명령어로 아래와 같이 release 정보를 확인할 수 있다.

```
+--------------------------+----------+-------------+
| Name                     | Versions | Commit Hash |
+--------------------------+----------+-------------+
| paasta-controller        | 2.0*     | 0f315314    |
+--------------------------+----------+-------------+
```

### 3. bosh deployment "paasta controller deployment 파일 이름"

### 4. bosh -n deploy

### bosh vms paasta-controller 명령어로 배포된 서비스들을 확인할 수 있다.

```
+---------------------------------------------------------------------------+---------+-----+-----------+------------+
| VM                                                                        | State   | AZ  | VM Type   | IPs        |
+---------------------------------------------------------------------------+---------+-----+-----------+------------+
| api_worker_z1/0 (89f5b1e4-ff10-46a2-908d-071c6595b0b5)                    | running | n/a | small_z1  | x.x.x.x    |
| api_z1/0 (32301eac-8689-48cb-aea5-6400ec7b8393)                           | running | n/a | medium_z1 | x.x.x.x    |
| blobstore_z1/0 (ae10f6f9-ce5d-4049-a02a-544fc452ce0b)                     | running | n/a | medium_z1 | x.x.x.x    |
| consul_z1/0 (84ec803a-23de-49bf-b6c9-d2f5d9863e11)                        | running | n/a | small_z1  | x.x.x.x    |
| doppler_z1/0 (0baa3f81-30e6-49ed-8a4d-838c0a0a300d)                       | running | n/a | small_z1  | x.x.x.x    |
| etcd_z1/0 (6e776c89-ee76-438c-bca1-2069277a6c95)                          | running | n/a | small_z1  | x.x.x.x    |
| ha_proxy_z1/0 (f6c843e8-d0c8-4d35-894e-9537ddf3645a)                      | running | n/a | router_z1 | x.x.x.x    |
| loggregator_trafficcontroller_z1/0 (6554c6eb-ca7e-4b62-a927-55dc2f8d883a) | running | n/a | small_z1  | x.x.x.x    |
| nats_z1/0 (409622ce-bb80-4b67-98d2-ea28b4063f84)                          | running | n/a | small_z1  | x.x.x.x    |
| postgres_z1/0 (f8eb7bd0-26bd-4234-986b-bb44b0deddde)                      | running | n/a | small_z1  | x.x.x.x    |
| router_z1/0 (7320abdc-1c2e-49b3-9dca-64963420c644)                        | running | n/a | router_z1 | x.x.x.x    |
| uaa_z1/0 (cdb39a37-12e3-47d9-8d05-dca07bf58ed5)                           | running | n/a | medium_z1 | x.x.x.x    |
+---------------------------------------------------------------------------+---------+-----+-----------+------------+

```
