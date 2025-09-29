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