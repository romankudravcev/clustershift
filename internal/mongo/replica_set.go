package mongo

import (
	"clustershift/internal/logger"
	"fmt"
	"strings"
	"time"
)

// AddMongoMember adds a new member to the MongoDB replica set using client pod
func AddMongoMember(client *Client, mongoHost, newHost string) error {
	script := fmt.Sprintf("rs.add('%s');", newHost)
	_, err := execMongoCommand(client, mongoHost, script)
	return err
}

// RemoveMongoMember removes a member from the MongoDB replica set using client pod
func RemoveMongoMember(client *Client, mongoHost, hostToRemove string) error {
	script := fmt.Sprintf(`rs.remove("%s");`, hostToRemove)
	logger.Info(mongoHost + " - removing" + hostToRemove + " from replica set")
	_, err := execMongoCommand(client, mongoHost, script)
	return err
}

// PromoteMember promotes a MongoDB replica set member to primary using client pod
func PromoteMember(client *Client, mongoHost, host string) error {
	return setPriorityForMember(client, mongoHost, host, highPriority)
}

// DemoteMember demotes a MongoDB replica set member from primary using client pod
func DemoteMember(client *Client, mongoHost, host string) error {
	return setPriorityForMember(client, mongoHost, host, lowPriority)
}

// setPriorityForMember sets the priority for a MongoDB replica set member using client pod
func setPriorityForMember(client *Client, mongoHost, host string, priority int) error {
	script := fmt.Sprintf(`
cfg = rs.conf();
for (var i = 0; i < cfg.members.length; i++) {
  if (cfg.members[i].host == "%s") {
    cfg.members[i].priority = %d;
  }
}
rs.reconfig(cfg, {force: true});
`, host, priority)

	_, err := execMongoCommand(client, mongoHost, script)
	return err
}

func InitReplicaSet(client *Client, mongoHost string) error {
	script := `
cfg = {
  _id: "rs0",
  members: [
	{ _id: 0, host: "` + mongoHost + `", priority:
 1 }
  ]
};
rs.initiate(cfg);
`
	_, err := execMongoCommandWithoutUser(client, mongoHost, script)
	if err != nil {
		return fmt.Errorf("failed to initiate MongoDB replica set: %w", err)
	}
	logger.Info("MongoDB replica set initiated successfully")
	return nil
}

// WaitForMongoMemberSecondary waits for a MongoDB member to become SECONDARY using client pod
func WaitForMongoMemberSecondary(client *Client, mongoHost, targetHost string) error {
	timeout := defaultTimeout
	interval := defaultCheckInterval
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ready, err := isMongoMemberSecondary(client, mongoHost, targetHost)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("member %s did not become SECONDARY within %v", targetHost, timeout)
}

// OverwriteMongoHosts updates the MongoDB replica set configuration with new hosts using client pod
func OverwriteMongoHosts(client *Client, mongoHost string, newHosts []string) error {
	// Build the script to update all hosts to use the new hostnames
	script := `
cfg = rs.conf();
var newHosts = [`

	for i, host := range newHosts {
		if i > 0 {
			script += ","
		}
		script += fmt.Sprintf(`"%s"`, host)
	}

	script += `];

for (var i = 0; i < cfg.members.length && i < newHosts.length; i++) {
  cfg.members[i].host = newHosts[i];
}

rs.reconfig(cfg, { force: true });`

	_, err := execMongoCommand(client, mongoHost, script)
	return err
}

// WaitForTargetPrimaryElection waits for a primary to be elected from target Cluster using client pod
func WaitForTargetPrimaryElection(client *Client, ctx *MigrationContext) error {
	// Use the first available host for checking primary status
	var checkHost string
	if len(ctx.TargetHosts) > 0 {
		checkHost = ctx.TargetHosts[0]
	} else if len(ctx.OriginHosts) > 0 {
		checkHost = ctx.OriginHosts[0]
	} else {
		return fmt.Errorf("no hosts available for checking primary status")
	}

	timeout := defaultTimeout
	interval := defaultCheckInterval
	deadline := time.Now().Add(timeout)

	logger.Info("Waiting for new primary to be elected from target Cluster...")

	for time.Now().Before(deadline) {
		primaryHost := strings.Split(checkHost, ":")[0]
		newPrimary, err := GetPrimaryMongoHost(client, primaryHost)
		if err != nil {
			logger.Debug(fmt.Sprintf("Could not determine current primary: %v, retrying...", err))
			time.Sleep(interval)
			continue
		}

		// Check if the new primary is one of our target hosts
		for _, targetHost := range ctx.TargetHosts {
			if strings.Contains(newPrimary, strings.Split(targetHost, ".")[0]) {
				logger.Info(fmt.Sprintf("New primary elected successfully from target Cluster: %s", newPrimary))
				return nil
			}
		}

		logger.Debug(fmt.Sprintf("Current primary %s is not from target Cluster, waiting...", newPrimary))
		time.Sleep(interval)
	}

	return fmt.Errorf("new primary was not elected from target Cluster within %v", timeout)
}
