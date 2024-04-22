# gaia
multi-cluster management


## Chart Repo
Add the following repo to use the chart:
```console
helm repo add hyperos http://122.96.144.180:30088/charts/hyperos
```

## Install Configuration

```bash
# parameter
common:
  clusterName: cluster1
  # Start different controllers by specifying the cluster level, and decide whether to install gaia-scheduler.
  # value: 'cluster', 'field' or 'global'
  clusterLevel: cluster
  mcSource: prometheus
  # Prometheus address of the current cluster
  promUrlPrefix: "http://prometheus-kube-prometheus-hypermoni.hypermonitor:9090"
  useHypernodeController: true
  topoSyncBaseUrl: "http://ssiexpose.synccontroller.svc:8080"
  networkBindUrl: http://nbi.domain1.svc.cluster.local/v1.0/network/scheme        # only on field level
  resourceBindingMergePostURL: http://192.168.101.73:37100/api/server/preScheduleSchemeReceiver
  # get label from hypernode if true, or get labels from node 
  useNodeRoleSelector: true
  aliyunSourceSite: '[{"content":"47.111.131.212","type":"ipaddr","priority":"20","port":80,"weight":"10"}]'



# example
## upgrade or install if not exist
helm upgrade --install -n gaia-system gaia hyperos/gaia --version=2.1.0-alpha \
  --set common.clusterLevel=global  \
  --set common.topoSyncBaseUrl=http://192.168.101.11:31555
```

## Deploy

### global
Run the following command in global cluster
```console
helm repo update hyperos
helm install -n gaia-system gaia hyperos/gaia --create-namespace --version=2.1.0-alpha  \
  --set common.clusterLevel=global
```

### field
Run the following command in field cluster
```console
helm repo update hyperos
helm install -n gaia-system gaia hyperos/gaia --create-namespace --version=2.1.0-alpha \
  --set common.clusterLevel=field
```

### cluster
Run the following command in cluster cluster
```console
helm repo update hyperos
helm install -n gaia-system gaia hyperos/gaia --create-namespace --version=2.1.0-alpha \
  --set common.clusterLevel=cluster
```

## Upgrade

If the crd changes, you need to upgrade or reinstall the crd.

### global
Run the following command in global cluster
```console
helm repo update hyperos
helm upgrade -n gaia-system gaia hyperos/gaia --version=2.1.0-alpha \
  --set common.clusterLevel=global
```

### filed
Run the following command in field cluster
```console
helm repo update hyperos
helm upgrade -n gaia-system gaia hyperos/gaia --version=2.1.0-alpha \
  --set common.clusterLevel=field
```


### cluster
Run the following command in cluster cluster
```console
helm repo update hyperos
helm upgrade -n gaia-system gaia hyperos/gaia --version=2.1.0-alpha \
  --set common.clusterLevel=cluster
```
