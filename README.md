# Dynomite on OpenShift as Statefulset

Dynomite (https://github.com/Netflix/dynomite) is a distributed dynamo layer for redis and memcached hight availability. It is use to replicate each service with data and that's mainly what we need with OpenShift statefulsets.

Note: memcached is not tested at this time.

The project includes 2 docker images:

- dagota: https://hub.docker.com/r/smilelab/dagota/ that is florida like server based on peer-finder (from kubernetes project). It search for pods in the statefulsets and setup a http server that can be used as florida provider in dynomite.
- dynomite: https://hub.docker.com/r/smilelab/dynomite/ that is a simple dynomite image containing a startup script that creates configuration

Both images are based on openshift/base-centos7 image to be able to start without any user rights problem on OpenShift.

Also, this images should be coupled with a Redis container that listens on 22122 port (invert of memcached port 11211). Pods should be created like the following example:

```yaml
containers:
- image: smilelab/dynomite
  name: dagota
  env:
  - name: POD_NAMESPACE
    valueFrom:
      fieldRef:
        apiVersion: v1
        fieldPath: metadata.namespace
  - name: SERVICE
    value: <name of the linked service>
- image: smilelab/dynomite
  name: dynomite
  ports:
  - containerPort: 6379
    protocol: TCP
- image: centos7/redis-32
  name: redis
  cmd:
  - redis-server
  - --port
  - "22122"
  - --protected-mode
  - no
  ports:
  - containerPort: 22122
    protocol: TCP
```

So that Pod has got 3 containers, one for dagota (to send detected peers to dynoite), dynomite that is used as redis proxy, and redis that is the service to use internally. **You should make a service to bind on dynomite port (6379) and not on 22122**, keep in mind that dynomite **is** the entrypoint.

Here is a Statefulset that should work:

```yaml
apiVersion: v1
items:
- apiVersion: apps/v1beta1
  kind: StatefulSet
  metadata:
    name: dynomite
    labels:
      app: dynomite
  spec:
    # note that name to setup service below
    serviceName: "dynomite"
    replicas: 3
    template:
      metadata:
        labels:
          app: dynomite
          deploymentconfig: dynomite
        annotations:
          pod.alpha.kubernetes.io/initialized: "true"
      spec:
        containers:
          - image: smilelab/dagota:devel
            name: dagota
            env:
              - name: POD_NAMESPACE
                valueFrom:
                  fieldRef:
                    apiVersion: v1
                    fieldPath: metadata.namespace
              # note that name to setup service below
              - name: DYN_SVC
                value: dynomite
          - image: smilelab/dynomite:devel
            name: dynomite
            ports:
            - containerPort: 6379
              protocol: TCP
          - image: centos/redis-32-centos7
            name: redis
            command:
              - "/opt/rh/rh-redis32/root/usr/bin/redis-server"
              - "--port"
              - "22122"
              - "--protected-mode"
              - "no"
            ports:
              - containerPort: 22122
                protocol: TCP

- apiVersion: v1
  kind: Service
  metadata:
    annotations:
      openshift.io/generated-by: OpenShiftNewApp
      service.alpha.kubernetes.io/tolerate-unready-endpoints: "true"
    creationTimestamp: null
    labels:
      app: dynomite
    # that name should be the same as
    # in serviceName in statefulset above
    name: dynomite
  spec:
    ports:
    - name: redis
      port: 6379
      protocol: TCP
      targetPort: 6379
    # IMPORTANT, clusterIP to NONE
    clusterIP: None
    selector:
      app: dynomite
      deploymentconfig: dynomite
kind: List
metadata: {}
```

**Important** You must use "dynomite:6379" (Service) from other pods to connect redis ! Don't connect to redis (22122).


# Notes to not break your init

- Be sure that serviceName and SERVICE env var in statefulset are the same value as the Service name
- Be sure that clusterIP in service is set to None
- Check that redis is listening on 22122


# TODO

- [ ] Better dagota go code
- [ ] Test memcached
