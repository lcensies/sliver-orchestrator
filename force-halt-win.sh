#!/bin/bash
# Windows VM force poweroff — WinRM hang edano
echo "Force powering off Windows-Target-Hasib-v3..."
VBoxManage controlvm "Windows-Target-Hasib-v3" poweroff 2>/dev/null && echo "Done" || echo "Already off"
