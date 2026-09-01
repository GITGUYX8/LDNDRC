# Testing Host Machine NVIDIA Drivers

**Purpose**: Verify that a host laptop has a working NVIDIA driver (minimum v535, recommended v550+)
before attempting the LDNDRC onboarding flow. This is critical because `join-cluster.sh`
(and its replacement `ldndrc-join`) requires a functioning driver for GPU time-slicing and
container access.

> **Rule of thumb**: `nvidia-smi` is the only check that proves the driver works.
> `lspci` only proves a GPU *exists*.

---

## Linux hosts

### The one-liner (main check)

```bash
nvidia-smi --query-gpu=name,driver_version --format=csv && echo "PASS: driver works" \
  || echo "FAIL: no working NVIDIA driver"
```

### Version gate (need >= 535, recommend >= 550)

```bash
v=$(nvidia-smi --query-gpu=driver_version --format=csv,noheader | cut -d. -f1)
[ "$v" -ge 550 ] && echo "PASS: driver $v" || echo "TOO OLD: driver $v (install 550+)"
```

### If `nvidia-smi` fails, diagnose

```bash
# 1. Is the driver package even installed?
dpkg -l | grep -E "nvidia-driver|nvidia-kernel" || echo "driver package not installed"

# 2. Installed but not loaded? → Secure Boot / MOK issue
mokutil --sb-state
lsmod | grep nvidia || echo "kernel module NOT loaded → MOK enrollment needed (reboot, enroll key) or disable Secure Boot"

# 3. What does Ubuntu recommend for this GPU?
ubuntu-drivers devices
```

### Container toolkit (needed later for k3s; ldndrc-join installs if missing)

```bash
nvidia-container-toolkit --version 2>/dev/null || echo "toolkit missing — will be installed by onboarding"
```

### Install driver (Ubuntu) (skip or uncomment as needed)

```bash
# sudo ubuntu-drivers install nvidia:550
# sudo reboot
# nvidia-smi
```

---

## Windows hosts

### The one-liner (PowerShell or CMD)

```powershell
nvidia-smi
```
If `nvidia-smi` is not recognized, the driver is not installed — download the latest Game Ready
or Studio driver from nvidia.com.

### More detail (PowerShell)

```powershell
Get-CimInstance win32_VideoController | Select-Object Name, DriverVersion
```

### For the WSL2 host path

```powershell
wsl --status
wsl -l -v
```
Then *inside* WSL2: `nvidia-smi` must work — it passes through from the Windows driver.
**Do NOT** install a Linux NVIDIA driver inside WSL2.

### Install if missing

Download Game Ready or Studio driver from https://www.nvidia.com/drivers
(any recent version is fine for WSL2).

---

## Interpretation guide

| Symptom | Meaning | Fix |
|---|---|---|
| `nvidia-smi` prints GPU + version | PASS — ready | nothing |
| command not found | no driver | `sudo ubuntu-drivers install nvidia:550` (Linux) / download from nvidia.com (Windows) |
| `couldn't communicate with the NVIDIA driver` | installed, module blocked | Secure Boot/MOK — enroll key on reboot or disable Secure Boot |
| driver 470 / 525 | too old | upgrade to 550+ branch |
| works on Windows, fails in WSL2 | WSL setup issue | `wsl --update`; do **not** install Linux driver inside WSL2 |