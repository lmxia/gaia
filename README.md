# gaia
multi-cluster management


## Chart Repo
Add the following repo to use the chart:
```console
helm repo add hyperos http://122.96.144.180:30088/charts/hyperos
```


## Deploy

### global or field
Run the following command in global or field cluster
```console
helm install -n gaia-system gaia hyperos/gaia --create-namespace --version=2.0.1
```

### cluster
Run the following command in cluster cluster
```console
helm install -n gaia-system gaia hyperos/gaia --create-namespace --version=2.0.1 --set isCluster=true
```