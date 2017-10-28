# Dynomite on OpenShift as Statefulset

Dynomite (https://github.com/Netflix/dynomite) is a distributed dynamo layer for redis and memcached hight availability. It is use to replicate each service with data and that's mainly what we need with OpenShift statefulsets.

Note: memcached is not tested at this time.

The project includes 2 docker images:

- dagota: https://hub.docker.com/r/smilelab/dagota/ that is florida like server based on peer-finder (from kubernetes project). It search for pods in the statefulsets and setup a http server that can be used as florida provider in dynomite.
- dynomite: https://hub.docker.com/r/smilelab/dynomite/ that is a simple dynomite image containing a startup script that creates configuration

Both images are based on openshift/base-centos7 image to be able to start without any user rights problem on OpenShift.

Also, this images should be coupled with a Redis container that listens on 22122 port (invert of memcached port 11211). 

So that Pod has got 3 containers, one for dagota (to send detected peers to dynoite), dynomite that is used as redis proxy, and redis that is the service to use internally. **You should make a service to bind on dynomite port (6379) and not on 22122**, keep in mind that dynomite **is** the entrypoint.

You can try example at https://raw.githubusercontent.com/Smile-SA/dagota/master/dynomite.statefulset.test.yml:

```bash
oc create -f https://raw.githubusercontent.com/Smile-SA/dagota/master/dynomite.statefulset.persistent.yml
# or, without storage
oc create -f https://raw.githubusercontent.com/Smile-SA/dagota/master/dynomite.statefulset.yml
```

**Important** You must use "dynomite:6379" (Service) from other pods to connect redis ! Don't connect to redis (22122).

Now you can try to SET/GET values:

```
$ oc run -it --name=redis-cli --image=redis --restart=Never -- bash
> redis-cli -h dynomite set foo "bar"
OK
> redis-cli -h dynomite get foo
"bar"
> exit

# then remove that pod
oc delete pod redis-cli --now
```


# Notes to not break your init

- Be sure that serviceName and SERVICE env var in statefulset are the same value as the Service name
- Be sure that clusterIP in service is set to None
- Check that redis is listening on 22122


# TODO

- [ ] Better dagota go code
- [ ] Test memcached
- [ ] Check why centos/redis-32-centos7 fails some connection while library/redis is ok
- [x] livenessProbe and readinessProbe
- [x] Volumes templates for redis


