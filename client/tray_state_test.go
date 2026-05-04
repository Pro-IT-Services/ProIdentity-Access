package main

import "testing"

func TestTrayHidesManagedDaemonTunnelsFromStandaloneSection(t *testing.T) {
	app := &App{
		mSessions: map[string]*activeSession{
			"server-1": {tunnelID: "daemon-managed-1"},
		},
		mUserConfigTunnels: map[string]string{
			"uconf:server-2": "daemon-user-config-1",
		},
	}

	hidden := app.trayDaemonTunnelIDsHiddenFromStandalone()
	for _, id := range []string{"daemon-managed-1", "daemon-user-config-1"} {
		if !hidden[id] {
			t.Fatalf("expected daemon tunnel %q to be hidden from standalone tray section; hidden=%#v", id, hidden)
		}
	}
	if hidden["standalone-1"] {
		t.Fatalf("standalone tunnel was hidden unexpectedly; hidden=%#v", hidden)
	}
}
