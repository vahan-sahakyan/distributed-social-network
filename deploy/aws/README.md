# AWS: one EC2 node

The prod env (gitops `bootstrap/root.yaml`) on a single `m7i-flex.large` running k3s, in an AWS Free plan account paid from credits. No load balancer, NAT or Elastic IP; the app answers on the node's public IP over https with a self-signed certificate.

| Piece | Choice | Why |
|---|---|---|
| Node | `m7i-flex.large`: 2 vCPU, 8 GB, x86 | largest Free plan type; no CPU credits to run out of. The prod profile (search and events on, ClickHouse capped at 1.5Gi; obs and tools off) idles at ~0.3 core and ~5.6 GiB. Events stay on: ClickHouse is the feed's only durable source |
| Disk | 30 GB gp3, encrypted | OS, images and every PVC (k3s local-path) |
| Network | own VPC `10.1.0.0/16`, one public subnet | free; nothing shared with other stacks |
| Address | one auto-assigned public IPv4 | changes on stop/start, but bills nothing while stopped. Login redirects are relative and `dsn-tls` is remade for the new IP at boot, so nothing in git holds the IP |
| Ingress | k3s Traefik as the Gateway, servicelb on host 80/443 | no cloud controller runs in k3s, so a LoadBalancer Service never creates an AWS LB |
| Access | SSH from one /32; the Kubernetes API through an ssh tunnel | nothing but 80/443 is public |
| Guardrail | Budget `dsn-credit-burn`, $100/month gross (credits excluded), account-wide, actual + forecast alerts | net cost stays $0 on the Free plan, so only gross cost shows the burn |

Cost, gross, from credits: ~$70/month compute + ~$3.65 IPv4 + ~$2.40 disk = ~$76/month running; ~$2.40/month stopped.

## Rules
- State the cost of any new AWS resource before adding it
- `terraform apply` only after reviewing the plan; `terraform destroy` wipes every volume
- Never join AWS Organizations, Control Tower or the Partner Network: each upgrades a Free plan account to paid
- The node keeps no data outside its root volume: replacing it (a new AMI, an edited `user_data.sh`) starts from empty, so both are in `ignore_changes`

## Up
Needs `terraform`, AWS credentials for the account, and the app's images for amd64 (the Publish workflow builds both archs).

```sh
cd deploy/aws
cp terraform.tfvars.example terraform.tfvars   # account id, alert email, your ip /32
terraform init
terraform plan -out=dsn.tfplan
terraform apply dsn.tfplan
./bootstrap.sh
```

`bootstrap.sh` is safe to rerun. It installs k3s (pinned), a boot-time unit that writes `dsn-tls` for the current IP, OpenBao's unseal key and first-start values, Argo CD with `bootstrap/argocd-config.yaml`, and the root app; then writes `kubeconfig` (through an ssh tunnel) and drops OpenBao's seed once it has started. Argo CD brings the app up on `https://<public ip>` in ~15 minutes.

The unseal key and the prod values are generated once into `~/.config/dsn-aws/` (override with `SECRETS=`). Keep them offline: without the key OpenBao's data can't be unsealed.

## Day to day
```sh
ssh -i dsn.pem -fN -L 16443:127.0.0.1:6443 ec2-user@$(terraform output -raw public_ip)
export KUBECONFIG=$PWD/kubeconfig
kubectl -n argocd get applications
kubectl -n argocd port-forward svc/argocd-server 9443:443   # admin / argocd-initial-admin-secret
```

Stop when idle, start when needed; the IP changes, so take the new one from `terraform refresh && terraform output public_ip` (and refresh `ssh_cidr` if yours changed):
```sh
aws ec2 stop-instances --instance-ids $(terraform output -raw instance_id)
aws ec2 start-instances --instance-ids $(terraform output -raw instance_id)
```
