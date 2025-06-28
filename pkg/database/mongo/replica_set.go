package mongo

import (
	"bytes"
	"clustershift/internal/kube"
	"fmt"
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

// stepDownMongoPrimary forces the current primary to step down
func stepDownMongoPrimary(c kube.Cluster, podName, namespace string) error {
	var out, errOut bytes.Buffer

	command := []string{mongoshCommand, "--eval", fmt.Sprintf("rs.stepDown(%d, %d, true);", stepDownDuration, stepDownDuration)}
	err := c.ExecIntoPod(namespace, podName, command, &out, &errOut)
	if err != nil {
		return fmt.Errorf("failed to step down MongoDB primary: %w, stderr: %s", err, errOut.String())
	}
	if errOut.Len() > 0 {
		return fmt.Errorf("mongosh error during step down: %s", errOut.String())
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
