# oklogging
k8s logging agent and server

# Agent
The agent countinue reads all log files in the specified directory and streams its to sever through TCP connection (one connection per log file).

Example k8s spec for agent deploy:
```
apiVersion: apps/v1beta2
kind: DaemonSet
metadata:
  name: oklogging-agent
spec:
  selector:
    matchLabels:
      name: oklogging-agent
  template:
    metadata:
      labels:
        name: oklogging-agent
    spec:
      containers:
      - image: {{docker_registry}}/oklogging-agent:0.1
        command: ["/oklogging-agent", "-server", "192.168.100.100:6600", "-offsets-dir", "/offsets", "-containers-dir", "/var/lib/docker/containers"]
        imagePullPolicy: Always
        name: okagent
        resources:
          requests:
            memory: "256Mi"
            cpu: "200m"
          limits:
            memory: "2Gi"
            cpu: "1"
        volumeMounts:
          - name: dockersocket
            mountPath: /var/run/docker.sock
            readOnly: true
          - name: containers
            mountPath: /var/lib/docker/containers
          - name: offsets
            mountPath: /offsets
      volumes:
        - hostPath:
            path: /var/lib/docker/containers/
          name: containers
        - hostPath:
            path: /var/run/docker.sock
          name: dockersocket
        - name: offsets
          emptyDir: {}
```

## Server

The server listens TCP port, accepts connections from agents and writing received data to files. The server also can make garbage collection (remove files older than X days).
