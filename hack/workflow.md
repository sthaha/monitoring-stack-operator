

operator image:
bundle image:
catalog image:

channels:
  - nightly
  - stable

workflows:
 - nightly
 - release


Nightly
------------
operator: foobar:<date>
bundle: foobar-bundle:<date>
channel: nightly

catalog image:  foobar-catalog:latest
from index: -- foobar-catalog:latest

Release
------------
operator: foobar:tag
bundle: foobar-bundle:tag
channel: stable

catalog image:  foobar-catalog:latest
catalog image:  foobar-catalog:<version>
  from index: -- foobar-catalog:<prev-version>


```

apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  annotations:
  name: monitoring-operators
  namespace: openshift-marketplace
spec:
  displayName: Monitoring Test Operator
  icon:
    base64data: ""
    mediatype: ""
  image: quay.io/sthaha/monitoring-operator-stack-catalog:latest
  publisher: Sunil Thaha
  sourceType: grpc
  updateStrategy:
    registryPoll:
      interval: 15m0s
```

