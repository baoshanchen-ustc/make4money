[CmdletBinding()]
param(
    [switch]$SkipFrontend,
    [switch]$SkipBuild,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$bashScript = Join-Path $scriptDir "deploy-zero-downtime.sh"

if (-not (Test-Path $bashScript)) {
    Write-Error "Cannot find deploy script: $bashScript"
    exit 1
}

function Normalize-ConfigValue {
    param(
        [AllowNull()]
        [string]$Value
    )

    if ($null -eq $Value) {
        return ""
    }

    return $Value.Trim().Trim("`r")
}

function Read-DeployConfig {
    param(
        [string]$ScriptDirPath
    )

    $config = @{
        REMOTE_HOST = "YOUR_SERVER_IP"
        REMOTE_USER = "root"
        BINARY_NAME = "code80"
        SSH_KEY = ""
    }

    $configPath = Join-Path $ScriptDirPath "deploy.local.conf"
    if (-not (Test-Path $configPath)) {
        return $config
    }

    foreach ($line in Get-Content -Encoding UTF8 $configPath) {
        $trimmed = $line.Trim()
        if (-not $trimmed -or $trimmed.StartsWith("#")) {
            continue
        }

        if ($trimmed -match '^\s*([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+?)\s*$') {
            $name = $Matches[1]
            $rawValue = $Matches[2]

            if ($rawValue -match '^"(.*)"(?:\s+#.*)?$') {
                $config[$name] = Normalize-ConfigValue -Value $Matches[1]
                continue
            }

            if ($rawValue -match "^'(.*)'(?:\s+#.*)?$") {
                $config[$name] = Normalize-ConfigValue -Value $Matches[1]
                continue
            }

            $cleanValue = ($rawValue -split '\s+#', 2)[0]
            $config[$name] = Normalize-ConfigValue -Value $cleanValue
        }
    }

    return $config
}

function Invoke-RemoteTempCleanup {
    param(
        [string]$ScriptDirPath
    )

    $config = Read-DeployConfig -ScriptDirPath $ScriptDirPath
    $remoteHost = Normalize-ConfigValue -Value $config.REMOTE_HOST
    $remoteUser = Normalize-ConfigValue -Value $config.REMOTE_USER
    $binaryName = Normalize-ConfigValue -Value $config.BINARY_NAME
    $sshKey = Normalize-ConfigValue -Value $config.SSH_KEY

    if (-not $remoteHost -or $remoteHost -eq "YOUR_SERVER_IP") {
        Write-Host "[WARNING] Skip temp cleanup: REMOTE_HOST is not configured."
        return
    }
    if (-not $remoteUser) {
        $remoteUser = "root"
    }
    if (-not $binaryName) {
        $binaryName = "code80"
    }

    $sshCommand = Get-Command ssh -ErrorAction SilentlyContinue
    if (-not $sshCommand) {
        Write-Host "[WARNING] Skip temp cleanup: local ssh command was not found."
        return
    }

    $sshArgs = @(
        "-o", "ConnectTimeout=10",
        "-o", "ServerAliveInterval=15",
        "-o", "ServerAliveCountMax=6"
    )

    if ($sshKey) {
        $sshArgs += "-i"
        $sshArgs += $sshKey
    }

    $remoteTarget = "$remoteUser@$remoteHost"
    $safePattern = "$binaryName.new.*"
    $remoteCommand = "find /tmp -maxdepth 1 -type f -name '$safePattern' -mmin +30 -print -delete 2>/dev/null; true"

    Write-Host "[INFO] Cleaning old temp files on server: /tmp/$binaryName.new.* (keep recent 30 minutes)"
    & $sshCommand.Source @sshArgs $remoteTarget $remoteCommand
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[WARNING] Temp cleanup failed (deployment result is unchanged)."
    }
}

function Resolve-GitBashPath {
    param(
        [string]$OverridePath
    )

    if ($OverridePath) {
        if (Test-Path $OverridePath) {
            return $OverridePath
        }
        throw "YI_CODE_GIT_BASH points to a missing file: $OverridePath"
    }

    $candidates = @(
        (Join-Path $env:ProgramFiles "Git\\bin\\bash.exe"),
        (Join-Path $env:ProgramFiles "Git\\usr\\bin\\bash.exe"),
        (Join-Path $env:ProgramW6432 "Git\\bin\\bash.exe"),
        (Join-Path $env:ProgramW6432 "Git\\usr\\bin\\bash.exe"),
        (Join-Path $env:LocalAppData "Programs\\Git\\bin\\bash.exe"),
        (Join-Path $env:LocalAppData "Programs\\Git\\usr\\bin\\bash.exe")
    ) | Where-Object { $_ -and (Test-Path $_) } | Select-Object -Unique

    if ($candidates.Count -gt 0) {
        return $candidates[0]
    }

    $bashCommand = Get-Command bash -ErrorAction SilentlyContinue
    if ($bashCommand -and (Split-Path $bashCommand.Source -Leaf) -ieq "bash.exe" -and $bashCommand.Source -match "\\Git\\") {
        return $bashCommand.Source
    }

    throw "Git Bash not found. Install Git for Windows, or set YI_CODE_GIT_BASH to bash.exe."
}

try {
    $bashExe = Resolve-GitBashPath -OverridePath $env:YI_CODE_GIT_BASH
} catch {
    Write-Error $_
    exit 1
}

$resolvedScript = (Resolve-Path $bashScript).Path -replace "\\", "/"
$bashArgs = @($resolvedScript)

if ($SkipFrontend) {
    $bashArgs += "--skip-frontend"
}
if ($SkipBuild) {
    $bashArgs += "--skip-build"
}
if ($Help) {
    $bashArgs += "--help"
}
if ($args.Count -gt 0) {
    $bashArgs += $args
}

& $bashExe @bashArgs
$deployExitCode = $LASTEXITCODE

if ($deployExitCode -eq 0 -and -not $Help) {
    try {
        Invoke-RemoteTempCleanup -ScriptDirPath $scriptDir
    } catch {
        Write-Host "[WARNING] Temp cleanup raised an exception (deployment result is unchanged): $($_.Exception.Message)"
    }
}

exit $deployExitCode
