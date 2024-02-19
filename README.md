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
  mcSource: prometheus
  # Prometheus address of the current cluster
  promUrlPrefix: "http://prometheus-kube-prometheus-hypermoni.hypermonitor:9090"
  useHypernodeController: true
  topoSyncBaseUrl: "http://ssiexpose.synccontroller.svc:8080"
  # networkBindUrl: only on field level
  networkBindUrl: http://nbi.domain1.svc.cluster.local/v1.0/network/scheme
  # resourceBindingMergePostURL: HyperOM interface, used to accept scheduling results.
  resourceBindingMergePostURL: http://192.168.101.73:37100/api/server/preScheduleSchemeReceiver
  useNodeRoleSelector: true
  aliyunSourceSite: '[{"content":"47.111.131.212","type":"ipaddr","priority":"20","port":80,"weight":"10"}]'


# example
helm install or upgrade -n gaia-system gaia hyperos/gaia --version=2.1.0-alpha \
  --set common.topoSyncBaseUrl=http://192.168.101.11:31555
  --set isCluster=true
```

## Deploy

### global or field
Run the following command in global or field cluster
```console
helm repo update hyperos
helm install -n gaia-system gaia hyperos/gaia --create-namespace --version=2.1.0-alpha
```

### cluster
Run the following command in cluster cluster
```console
helm repo update hyperos
helm install -n gaia-system gaia hyperos/gaia --create-namespace --version=2.1.0-alpha --set isCluster=true
```

## Upgrade

If the crd changes, you need to upgrade or reinstall the crd.

### global or field
Run the following command in global or field cluster
```console
helm repo update hyperos
helm upgrade -n gaia-system gaia hyperos/gaia --version=2.1.0-alpha
```

### cluster
Run the following command in cluster cluster
```console
helm repo update hyperos
helm upgrade -n gaia-system gaia hyperos/gaia --version=2.1.0-alpha --set isCluster=true
```
