# NG-VOICE MAIN TEST
## Tasks
1. Deploy a kubernetes cluster, locally or on a public cloud.
2. Deploy a DB cluster on K8s with persistant data (MySQL or MariaDB).
3. Deploy a Web Server on K8s (Nginx, Apache, …) with the following conditions:
   1. use multiple replicas of the web-server pods
   2. The web-page should be accessible from the browser.
   3. Custom configuration of the webserver should be mounted and used in the pod.
   4. The web-page should:
      1. Show the Pod IP.
      2. include a field called "serving-host". This field should be modified in an init container to be "Host-{the last 5 character of the web-server pod name}" For EX. web-server pod name is `web-server-7f89cf47bf-25gxj` the web-page should show: `serving-host=Host-5gxj`
4. Suggest and implement a way to only allows the web server pods to initiate connections to the database pods on the correct port (e.g., 3306 for MySQL). All other traffic to the database should be denied.
5. Suggest a Disaster recovery solution for the DB.
6. Find a flexible way to connect the Pod to a new network other than the Pods networks with proper routes. no LoadBalancer service is needed.
7. Find a way to allow the deployment engineer to schedule specific replicas of the database cluster on specific k8s nodes.

## Golang application task
8. Write a Golang applications for monitoring the status of the above pods. This
application should print a log at any point in time if there is any of the following
changes:
 - a new pod has been created
 - pod has been deleted
 - pod has been updated
9. Use the Helm chart to deploy all above components

## Solution & Comments
### 1. Deploy a kubernetes cluster, locally or on a public cloud.
For this task, Ansible was used to provision a Linux machine from scratch.
The playbook automates the following steps:
- Adding the required users, installing needed utilities, changing SSH port to custom port 2022
- Installing vanila Kubernetes distribution<br>

Steps to install Kubernetes
1. Provision 3x Linux VM with Ubuntu 22.04
2. Update `inventory.yaml` file with correct IP
3. Run Ansible playbook to install K8s
```
ansible-playbook playbook.yaml
```

### 2. Deploy a DB cluster on K8s with persistant data (MySQL or MariaDB).
For this task, Bitnami Helm chart for MariaDB Galera was chosen.
By default, there is no PV/PVC in Helm chart by default, they needed to be created first.
Ansible playbook has been updated with tasks with tags `copy_manifests` and `run_database`.<br><br>
```
ansible-playbook playbook.yaml --tags copy_manifests,run_database
```
*I should be transparent here — I don’t have production experience running or supporting large-scale database clusters. My hands-on work with databases so far has been limited to smaller, secondary services where availability and performance requirements were not critical.*

*For this task, I simply researched how to deploy a clustered database and applied the basic configuration steps, but I haven’t gone deep into the operational nuances such as tuning, backup/restore strategies, or troubleshooting in production. I’m aware this is not my area of strong expertise yet.*

### 3. Deploy a Web Server on K8s
Custom config: NGINX uses `nginx.conf` from the ConfigMap.<br>
Init container: renders `index.html` into a shared `emptyDir` using:
- POD_IP from the Downward API (status.podIP), and
- HOSTNAME (pod name) to compute the suffix.<br>
Ansible playbook has been updated with tasks with tag `run_webserver`
```
ansible-playbook playbook.yaml --tags run_webserver
```
Check it
```
curl http://139.162.150.79:30080
curl http://139.162.150.196:30080
```

### 4. Suggest and implement a way to only allows the web server pods to initiate connections to the database pods
Need swap Flannel ==> Calico to enforce Network Policy
Run Ansible playbook to install and enable Calico
```
ansible-playbook playbook.yaml --tags calico
```
Network policy can be used to achive this task.
Ansible playbook has been updated with tasks with tag `run_webserver`
```
ansible-playbook playbook.yaml --tags network_policy
```
Check it from a web pod
```
kubectl run test-web \
  -n web \
  --rm -it \
  --image=busybox:1.36 \
  --restart=Never -- \
  sh
```
and test
```
nc -zv database-mariadb-galera.database.svc.cluster.local 3306
```
Check it from any other pod
```
kubectl run test-other \
  -n default \
  --rm -it \
  --image=busybox:1.36 \
  --restart=Never -- \
  sh
```
and test
```
nc -zv database-mariadb-galera.database.svc.cluster.local 3306
```
So it does not work as expectes ==> Helm chart has already deployed a network policy (they are additive “allow” rules) ==> need to change Helm chart network policy.<br><br>
Here is the official doc for helm chart values:
https://artifacthub.io/packages/helm/bitnami/mariadb-galera

Get passwords
```
export MARIADB_ROOT_PASSWORD=$(
  kubectl get secret -n database database-mariadb-galera \
    -o jsonpath='{.data.mariadb-root-password}' | base64 -d
)
export MARIADB_GALERA_MARIABACKUP_PASSWORD=$(
  kubectl get secret -n database database-mariadb-galera \
    -o jsonpath='{.data.mariadb-galera-mariabackup-password}' | base64 -d
)
```
Add network policy to values.yaml and update helm chart with new network policy settings from `values.yaml`
```
helm install database oci://registry-1.docker.io/bitnamicharts/mariadb-galera \
  -n database \
  -f values.yaml \
  --set rootUser.password="$MARIADB_ROOT_PASSWORD" \
  --set auth.rootPassword="$MARIADB_ROOT_PASSWORD" \
  --set galera.mariabackup.password="$MARIADB_GALERA_MARIABACKUP_PASSWORD" \
  --set image.registry=docker.io \
  --set image.repository=bitnamilegacy/mariadb-galera \
  --set image.tag=12.0.2-debian-12-r0
```
Now network policy has been updated according to the requirement.
```
kubectl describe networkpolicy database-mariadb-galera -n database
```

### 5. Suggest a Disaster recovery solution for the DB
I usually restore my databases from Persistent Volumes, which are replicated hourly to a backup storage location. However, this is only a service database used for IT infrastructure automation, not a production workload.<br>
For production databases, I would consider a more specialized backup and recovery solution that takes database specifics into account and stores consistent dumps in a reliable backup system. I don’t have hands-on experience with this yet. ChatGPT recommends tools such as Percona XtraBackup for that use case.<br>
I would create an **Ansible playbook** to automate the restore procedure and use a **Kubernetes CronJob** to schedule regular backups into reliable storage.


### 6. Find a flexible way to connect the Pod to a new network other than the Pods networks with proper routes
I haven’t deployed this kind of multi-network setup in production, because there was no real business need. However, I’ve always been interested in exploring it in more detail. As I can see, **multus** is a flexible way to connect Pods to external networks with proper routing.
https://github.com/k8snetworkplumbingwg/multus-cni


### 7. Find a way to allow the deployment engineer to schedule specific replicas of the database cluster on specific k8s nodes
1. **Manual**
```
kind: Deployment
spec:
    spec:
      nodeName: node02
```
2. **Node Selector**
```
kubectl label nodes node01 ssdtype=slow
kubectl label nodes node02 ssdtype=fast
```
change Deployment
```
kind: Deployment
spec:
    nodeSelector:
      ssdtype: fast
```
3. **Node Affinity** (more flexible than Node Selector)
```
affinity:
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: disktype
          operator: In
          values:
          - ssd
        - key: zone
          operator: NotIn
          values:
          - zoneA
```
4. **Taint and Toleration**<br>
It is a flexible way to assign pods to nodes, for example can be used to reserve a node for DB pods only.

### 8. Write a Golang application
Install and run script
```
curl -LO https://go.dev/dl/go1.22.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.5.linux-amd64.tar.gz
echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.bashrc
export PATH=/usr/local/go/bin:$PATH
go mod init podwatch
go get k8s.io/api@v0.30.0
go get k8s.io/apimachinery@v0.30.0
go get k8s.io/client-go@v0.30.0
go mod tidy
go build -o podwatch .
./podwatch -kubeconfig ~/.kube/config -namespace database
```

Result
```
admin@control:~/code$ ./podwatch -kubeconfig ~/.kube/config -namespace database
2025-09-30T13:50:03.036Z | CREATED  pod database/database-mariadb-galera-0 phase=Running ip=10.244.104.2 node=node2
2025-09-30T13:50:03.037Z | CREATED  pod database/database-mariadb-galera-1 phase=Pending ip=10.244.166.137 node=node1
2025-09-30T13:50:03.123Z | watching pods (namespace=database, started=2025-09-30T13:50:03Z)
2025-09-30T13:51:17.970Z | UPDATED  pod database/database-mariadb-galera-1 phase:Pending->Pending ip:->10.244.166.137 node:->node1
2025-09-30T13:51:18.115Z | UPDATED  pod database/database-mariadb-galera-1 phase:Pending->Pending ip:10.244.166.137-> node:->node1
2025-09-30T13:51:18.938Z | UPDATED  pod database/database-mariadb-galera-1 phase:Pending->Pending ip:->10.244.166.137 node:->node1
2025-09-30T13:51:18.963Z | UPDATED  pod database/database-mariadb-galera-1 phase:Pending->Failed ip:->10.244.166.137 node:->node1
2025-09-30T13:51:18.999Z | DELETED  pod database/database-mariadb-galera-1
2025-09-30T13:51:19.019Z | CREATED  pod database/database-mariadb-galera-1 phase=Pending ip= node=
2025-09-30T13:51:19.067Z | UPDATED  pod database/database-mariadb-galera-1 phase:Pending->Pending ip:-> node:->node1
2025-09-30T13:51:19.085Z | UPDATED  pod database/database-mariadb-galera-1 phase:Pending->Pending ip:-> node:->node1
2025-09-30T13:51:19.512Z | UPDATED  pod database/database-mariadb-galera-1 phase:Pending->Pending ip:-> node:->node1
2025-09-30T13:51:20.939Z | UPDATED  pod database/database-mariadb-galera-1 phase:Pending->Pending ip:->10.244.166.138 node:->node1
```

*I’m not familiar with the Go programming language, my main experience is with Python, which I actively use for automation and integration tasks. The Go solution I showed here was generated with the help of ChatGPT.*
