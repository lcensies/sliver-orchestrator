Vagrant.configure("2") do |config|

# ---------------------------------------------------------
# 1. Linux Victim: Pivot Host (Ubuntu)
# ---------------------------------------------------------
config.vm.define "linux_pivot" do |linux|
linux.vm.box = "ubuntu/jammy64"

# Change public_network to private_network as soon as possible
# Set the IP address to something that is not yours (example: 192.168.100.10)
linux.vm.network "private_network", ip: "192.168.100.10"

# Internal Network: sliver-lab (for talking to Windows)
linux.vm.network "private_network", ip: "172.16.1.10", virtualbox__intnet: "sliver-lab"

linux.vm.provider "virtualbox" do |vb|
vb.name = "Linux-Pivot-Hasib-v3" 
vb.memory = "1024" 
vb.cpus = 1 
end 
end 

# -------------------------------------------------------------- 
# 2. Windows Victim: Target Host 
# -------------------------------------------------------------- 
config.vm.define "win_target" do |win| 
win.vm.box = "win10-local" 

# Internal Network: sliver-lab (Lonux only) 
win.vm.network "private_network", ip: "172.16.1.20", virtualbox__intnet: "sliver-lab" 

win.vm.provider "virtualbox" do |vb| 
vb.name = "Windows-Target-Hasib-v3" 
vb.memory = "4096" 
vb.cpus = 2 
vb.gui = true 

vb.customize ["modifyvm", :id, "--vram", "128"] 
vb.customize ["modifyvm", :id, "--graphicscontroller", "vboxsvga"] 
vb.customize ["modifyvm", :id, "--accelerate3d", "on"] 
end 
end
end
