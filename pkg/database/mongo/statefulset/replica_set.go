package statefulset

import (
	"bytes"
	"clustershift/internal/kube"
	"clustershift/internal/logger"
	"fmt"
	"strings"
	"time"
)

// addMongoMember adds a new member to the MongoDB replica set
func addMongoMember(origin kube.Cluster, name, namespace, host string) error {
	script := fmt.Sprintf("rs.add('%s');", host)
	return execMongoCommand(origin, name, namespace, script)
}

// removeMongoMember removes a member from the MongoDB replica set
func removeMongoMember(cluster kube.Cluster, podName string, namespace string, host string) error {
	script := fmt.Sprintf(`rs.remove("%s");`, host)
	return execMongoCommand(cluster, podName, namespace, script)
}

// promoteMember promotes a MongoDB replica set member to primary
func promoteMember(c kube.Cluster, podName, namespace, host string) error {
	return setPriorityForMember(c, podName, namespace, host, highPriority)
}

// demoteMember demotes a MongoDB replica set member from primary
func demoteMember(c kube.Cluster, podName, namespace, host string) error {
	return setPriorityForMember(c, podName, namespace, host, lowPriority)
}

// setPriorityForMember sets the priority for a MongoDB replica set member
func setPriorityForMember(c kube.Cluster, podName, namespace, host string, priority int) error {
	var out, errOut bytes.Buffer

	script := fmt.Sprintf(`
cfg = rs.conf();
for (var i = 0; i < cfg.members.length; i++) {
  if (cfg.members[i].host == "%s") {
    cfg.members[i].priority = %d;
  }
}
rs.reconfig(cfg, {force: true});
`, host, priority)

	command := []string{mongoshCommand, "--eval", script}
	err := c.ExecIntoPod(namespace, podName, command, &out, &errOut)
	if err != nil {
		return fmt.Errorf("failed to set priority %d for member %s: %w, stderr: %s", priority, host, err, errOut.String())
	}
	if errOut.Len() > 0 {
		return fmt.Errorf("mongosh error while setting priority: %s", errOut.String())
	}

	return nil
}

// waitForMongoMemberSecondary waits for a MongoDB member to become SECONDARY
func waitForMongoMemberSecondary(c kube.Cluster, podName, namespace, host string) error {
	timeout := defaultTimeout
	interval := defaultCheckInterval
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ready, err := isMongoMemberSecondary(c, podName, namespace, host)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("member %s did not become SECONDARY within %v", host, timeout)
}

// overwriteMongoHosts updates the MongoDB replica set configuration with new hosts
func overwriteMongoHosts(c kube.Cluster, podName, namespace string, newHosts []string) error {
	var out, errOut bytes.Buffer

	script := "cfg = rs.conf();"
	for i, host := range newHosts {
		script += fmt.Sprintf(`cfg.members[%d].host = "%s";`, i, host)
	}
	script += "rs.reconfig(cfg);"

	command := []string{mongoshCommand, "--eval", script}

	err := c.ExecIntoPod(namespace, podName, command, &out, &errOut)
	if err != nil {
		return fmt.Errorf("failed to exec into pod: %w, stderr: %s", err, errOut.String())
	}
	if errOut.Len() > 0 {
		return fmt.Errorf("mongosh error: %s", errOut.String())
	}
	return nil
}

// waitForTargetPrimaryElection waits for a primary to be elected from target cluster
func waitForTargetPrimaryElection(c kube.Clusters, ctx *MigrationContext) error {
	// Try to get primary from target cluster first, fallback to origin
	var primaryCheckCluster kube.Cluster
	var podName string

	// If we have target hosts, try to use the first target host for checking
	if len(ctx.TargetHosts) > 0 {
		primaryCheckCluster = c.Target
		// Extract pod name from target host (assuming format like "pod-name.service.namespace")
		parts := strings.Split(ctx.TargetHosts[0], ".")
		if len(parts) > 0 {
			podName = parts[0]
		} else {
			podName = ctx.TargetHosts[0]
		}
	} else {
		primaryCheckCluster = c.Origin
		podName = ctx.PrimaryHost
	}

	timeout := defaultTimeout
	interval := defaultCheckInterval
	deadline := time.Now().Add(timeout)

	logger.Info("Waiting for new primary to be elected from target cluster...")

	for time.Now().Before(deadline) {
		newPrimary, err := getPrimaryMongoHost(primaryCheckCluster, podName, ctx.StatefulSet.Namespace)
		if err != nil {
			logger.Debug(fmt.Sprintf("Could not determine current primary: %v, retrying...", err))
			time.Sleep(interval)
			continue
		}

		// Check if the new primary is one of our target hosts
		for _, targetHost := range ctx.TargetHosts {
			if strings.Contains(newPrimary, strings.Split(targetHost, ".")[0]) {
				logger.Info(fmt.Sprintf("New primary elected successfully from target cluster: %s", newPrimary))
				return nil
			}
		}

		logger.Debug(fmt.Sprintf("Current primary %s is not from target cluster, waiting...", newPrimary))
		time.Sleep(interval)
	}

	return fmt.Errorf("new primary was not elected from target cluster within %v", timeout)
}
