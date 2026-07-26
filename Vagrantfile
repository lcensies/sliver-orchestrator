# Vagrantfile — Option B: fully self-contained VirtualBox lab
#
#   ┌────────────┐   192.168.56.0/24 (host-only, "attacker" net)
#   │    c2      │ .5 ──────────────┐
#   │ Sliver +   │                  │
#   │ scenario   │            ┌─────┴──────┐  192.168.56.10
#   └────────────┘            │ linux_pivot│ .10  (dual-homed gateway)
#                             │            │ ────────────┐
#                             └────────────┘             │ 172.16.1.0/24
#                                                         │ "sliver-lab" intnet (ISOLATED)
#                                                   ┌─────┴──────┐ 172.16.1.20
#                                                   │ win_target │  (hidden target — NO route to c2)
#                                                   └────────────┘
#
# Everything runs in ONE VirtualBox hypervisor. Kali (VMware) is NOT required at
# runtime — use it as an operator workstation only if you want, connecting to
# the c2 API/gRPC over the host-only network.
#
# Bring up in order (c2 first so victims can reach it):
#   vagrant up c2
#   vagrant up linux_pivot
#   vagrant up win_target
#
# RAM budget (host needs ~11 GB free for VMs):
#   c2 2048 + linux_pivot 1024 + win_target 4096 = ~7 GB guests

Vagrant.configure("2") do |config|

  config.ssh.insert_key = false
  config.ssh.private_key_path = ["~/.vagrant.d/insecure_private_key"]

  # ── 0. C2 / Orchestrator (Sliver + scenario-server) ─────────────────────────
  # Provisioned from lab/provision/c2-server.sh (systemd services).
  # The repo is synced into /sliver-repo so the locally-built binaries
  # (scenario-server, optionally sliver-server) and atomics are available.
  config.vm.define "c2" do |c2|
    c2.vm.box = "ubuntu/jammy64"
    c2.ssh.insert_key = false

    # Attacker-facing host-only network. C2 lives at .5 so it never collides
    # with linux_pivot (.10).
    c2.vm.network "private_network", ip: "192.168.56.5"

    # Sync the whole repo read-only into the VM. The provisioner copies out
    # the prebuilt binaries + atomics from here.
    c2.vm.synced_folder ".", "/sliver-repo", type: "virtualbox"

    c2.vm.provider "virtualbox" do |vb|
      vb.name = "C2-Orchestrator-Hasib"
      vb.memory = "2048"
      vb.cpus = 2
    end

    c2.vm.provision "shell" do |s|
      s.path = "lab/provision/c2-server.sh"
      s.env = {
        # C2 advertises this address to beacons + operator config.
        "C2_HOST"              => "192.168.56.5",
        "SCENARIO_PORT"        => "8080",
        # Atomics were synced with the repo submodule.
        "SCENARIO_ATOMICS_SRC" => "/sliver-repo/sliver-orchestrator-workspace/atomics",
      }
    end
  end

  # ── 1. Linux Victim: Pivot Host (Ubuntu, dual-homed) ────────────────────────
  config.vm.define "linux_pivot" do |linux|
    linux.vm.box = "ubuntu/jammy64"
    linux.ssh.insert_key = false

    # Reachable from the attacker/C2 network.
    linux.vm.network "private_network", ip: "192.168.56.10"
    # Isolated internal net toward the Windows target.
    linux.vm.network "private_network", ip: "172.16.1.10",
                     virtualbox__intnet: "sliver-lab"

    linux.vm.provider "virtualbox" do |vb|
      vb.name = "Linux-Pivot-Hasib-v3"
      vb.memory = "1024"
      vb.cpus = 1
    end

    # Auto-download + install the Linux beacon from the scenario API.
    # linux_pivot CAN reach the C2 (both on 192.168.56.0/24), so this works.
    linux.vm.provision "shell" do |s|
      s.path = "lab/provision/victim-linux.sh"
      s.env = { "C2_HOST" => "192.168.56.5" }
    end
  end

  # ── 2. Windows Victim: Target Host (isolated) ───────────────────────────────
  # NOTE: NO auto-implant provisioning. win_target has NO route to the C2 by
  # design — it only sits on the isolated 172.16.1.0/24 net. You deploy the
  # Windows implant THROUGH linux_pivot (SOCKS5 / port-forward) after you have a
  # session on the pivot. See SETUP.md "Deploying the Windows implant".
  config.vm.define "win_target" do |win|
    win.vm.box = "gusztavvargadr/windows-10"

    win.vm.network "private_network", ip: "172.16.1.20",
                   virtualbox__intnet: "sliver-lab"
    win.vm.network "private_network", ip: "192.168.56.20",
                   virtualbox__hostonly: "vboxnet0"

    win.vm.guest = :windows
    win.vm.graceful_halt_timeout = 30
    win.vagrant.plugins = []
    win.vm.communicator = "winrm"
    win.winrm.transport = :negotiate
    win.winrm.retry_limit = 30
    win.winrm.retry_delay = 10

    win.vm.provider "virtualbox" do |vb|
      vb.name = "Windows-Target-Hasib-v3"
      vb.customize ["modifyvm", :id, "--acpi", "on"]
      vb.memory = "6166"
      vb.cpus = 2
      vb.gui = true
    end

    # Optional: enable a WinRM helper (firewall/route prep) but DO NOT fetch an
    # implant here — the target cannot reach the C2 directly. Left commented so
    # the pivoting story stays intact.
    #
    win.vm.provision "shell" do |s|
      s.path = "lab/provision/victim-windows.ps1"
      s.env  = { "C2_HOST" => "192.168.56.5" }
    end
  end

end
