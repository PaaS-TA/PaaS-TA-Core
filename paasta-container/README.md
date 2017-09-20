# PaaS-TA Container Release

## Deploy

### paasta-container를 설치하는 절차는 아래와 같습니다.

### 1. bosh create release --force --name paasta-container --version 2.0
### 2. bosh upload release

### bosh releases 명령어로 아래와 같이 release 정보를 확인할 수 있다.

```
+--------------------------+----------+-------------+
| Name                     | Versions | Commit Hash |
+--------------------------+----------+-------------+
| paasta-container         | 2.0*     | b857e171    |
+--------------------------+----------+-------------+
```

### 3. bosh deployment "paasta container deployment 파일 이름"

### 4. bosh -n deploy

### bosh vms paasta-container 명령어로 배포된 서비스들을 확인할 수 있다.

```
+-----------------------------------------------------------+---------+-----+------------------+---------+
| VM                                                        | State   | AZ  | VM Type          | IPs     |
+-----------------------------------------------------------+---------+-----+------------------+---------+
| access_z1/0 (5482362a-8ae1-4f61-90d5-a55cb32ca8bb)        | running | n/a | access_z1        | x.x.x.x |
| brain_z1/0 (c801db67-e240-4f6a-bbd3-f834a7bb39db)         | running | n/a | brain_z1         | x.x.x.x |
| cc_bridge_z1/0 (fbacc525-3cbb-4589-a7c3-39ce248194a0)     | running | n/a | cc_bridge_z1     | x.x.x.x |
| cell_z1/0 (0fd087bd-e718-48b6-9168-ffb47e3343a8)          | running | n/a | cell_z1          | x.x.x.x |
| cell_z1/1 (ab590708-e321-445c-b86a-1bc5ee312485)          | running | n/a | cell_z1          | x.x.x.x |
| database_z1/0 (2dcc0aae-b176-49cd-ac88-52641e35a543)      | running | n/a | database_z1      | x.x.x.x |
| route_emitter_z1/0 (e4e4eafd-5d9d-4b3a-b0f6-3c0c451a2e38) | running | n/a | route_emitter_z1 | x.x.x.x |
+-----------------------------------------------------------+---------+-----+------------------+---------+
```
