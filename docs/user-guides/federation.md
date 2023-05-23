# Federate Monitoring Stack and Openshift In-Cluster Prometheus

## Architecture / Topology

![Architecture](federation/assets/cmo-obo-federation.svg)

This example deploy a MonitoringStack in `federate-cmo` namespace and reads
only a selected set of metrics from the in-cluster prometheus.

## Steps

### Deploy Monitoring Stack

![MonitoringStack](federation/manifests/10-ms.yaml)



### Grant permission to Federate In-Cluster Prometheus

![Grand Privileges](federation/manifests/11-crb.yaml)

### Create ServiceMonitor for Federation

![ServiceMonitor Prometheus](federation/manifests/20-smon-cmo.yaml)

## Validation
