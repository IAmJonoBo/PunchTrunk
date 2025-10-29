[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$BundlePath,
    [string]$InstallDir = (Join-Path -Path $env:ProgramData -ChildPath 'PunchTrunk'),
    [string]$EnvScript,
    [string]$ChecksumPath,
    [switch]$Force,
    [switch]$SkipCacheLink
)

function Invoke-ChecksumValidation {
    param(
        [string]$Bundle,
        [string]$Checksum
    )
    if (-not (Test-Path -Path $Checksum)) {
        throw "Checksum file not found at $Checksum"
    }
    $expected = (Get-Content -Path $Checksum).Split(' ', [System.StringSplitOptions]::RemoveEmptyEntries)[0]
    $actual = (Get-FileHash -Path $Bundle -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($expected.ToLowerInvariant() -ne $actual) {
        throw "Checksum mismatch for $Bundle. Expected $expected but computed $actual"
    }
}

if (-not (Test-Path -Path $BundlePath)) {
    throw "Bundle not found at $BundlePath"
}

if (-not (Get-Command tar -ErrorAction SilentlyContinue)) {
    throw "tar.exe is required to extract the bundle"
}

$bundleFull = (Resolve-Path -Path $BundlePath).Path
$installFull = (Resolve-Path -Path (New-Item -ItemType Directory -Path $InstallDir -Force)).Path

if (-not $EnvScript) {
    $EnvScript = Join-Path -Path $installFull -ChildPath 'punchtrunk-airgap.ps1'
}

if (-not $ChecksumPath) {
    $candidate = "$bundleFull.sha256"
    if (Test-Path -Path $candidate) {
        $ChecksumPath = $candidate
    }
}

if ($ChecksumPath) {
    Invoke-ChecksumValidation -Bundle $bundleFull -Checksum $ChecksumPath
}

$tempRoot = Join-Path -Path ([System.IO.Path]::GetTempPath()) -ChildPath ("punchtrunk-setup-" + [System.Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null

try {
    & tar -xzf $bundleFull -C $tempRoot | Out-Null

    $bundleDirs = Get-ChildItem -Path $tempRoot -Directory
    if ($bundleDirs.Count -ne 1) {
        throw "Expected bundle to contain a single root directory, found $($bundleDirs.Count)"
    }

    $bundleRoot = $bundleDirs[0]
    $releaseDir = Join-Path -Path $installFull -ChildPath $bundleRoot.Name

    if (Test-Path -Path $releaseDir) {
        if ($Force) {
            Remove-Item -Path $releaseDir -Recurse -Force
        }
        else {
            throw "Release directory already exists at $releaseDir. Use -Force to overwrite."
        }
    }

    Move-Item -Path $bundleRoot.FullName -Destination $releaseDir

    $currentLink = Join-Path -Path $installFull -ChildPath 'current'
    if (Test-Path -Path $currentLink) {
        Remove-Item -Path $currentLink -Force
    }
    New-Item -ItemType Junction -Path $currentLink -Target $releaseDir | Out-Null

    $binDir = Join-Path -Path $installFull -ChildPath 'bin'
    New-Item -ItemType Directory -Path $binDir -Force | Out-Null

    $punchBinary = Get-ChildItem -Path (Join-Path -Path $releaseDir -ChildPath 'bin') | Where-Object { $_.Name -like 'punchtrunk*' } | Select-Object -First 1
    if (-not $punchBinary) {
        throw "Could not locate punchtrunk binary inside the bundle"
    }

    $trunkBinary = Get-ChildItem -Path (Join-Path -Path $releaseDir -ChildPath 'trunk/bin') | Where-Object { $_.Name -like 'trunk*' } | Select-Object -First 1
    if (-not $trunkBinary) {
        throw "Could not locate trunk binary inside the bundle"
    }

    $punchWrapper = Join-Path -Path $binDir -ChildPath 'punchtrunk.cmd'
    $wrapperContent = "@echo off`r`nset \"PUNCHTRUNK_HOME=$currentLink\"`r`nset \"PUNCHTRUNK_TRUNK_BINARY=$currentLink\\trunk\\bin\\$($trunkBinary.Name)\"`r`nset \"PUNCHTRUNK_AIRGAPPED=1\"`r`nset \"PATH=$currentLink\\bin;$currentLink\\trunk\\bin;%PATH%\"`r`n\"$($punchBinary.FullName)\" %*`r`n"
    Set-Content -Path $punchWrapper -Value $wrapperContent -Encoding ASCII

    $trunkWrapper = Join-Path -Path $binDir -ChildPath 'trunk.cmd'
    $trunkContent = "@echo off`r`nset \"PATH=$currentLink\\trunk\\bin;%PATH%\"`r`n\"$($trunkBinary.FullName)\" %*`r`n"
    Set-Content -Path $trunkWrapper -Value $trunkContent -Encoding ASCII

    $cacheSource = Join-Path -Path $releaseDir -ChildPath 'trunk/cache'
    $cacheTarget = Join-Path -Path $installFull -ChildPath 'cache/trunk'
    $cacheCopied = $false
    if (Test-Path -Path $cacheSource) {
        if (Test-Path -Path $cacheTarget) {
            if ($Force) {
                Remove-Item -Path $cacheTarget -Recurse -Force
            }
            else {
                throw "Cache directory already exists at $cacheTarget. Use -Force to replace it."
            }
        }
        New-Item -ItemType Directory -Path $cacheTarget -Force | Out-Null
        & robocopy $cacheSource $cacheTarget /E /NFL /NDL /NJH /NJS /NC /NS /NP | Out-Null
        $cacheCopied = $true

        if (-not $SkipCacheLink.IsPresent) {
            $localCacheRoot = Join-Path -Path $env:LOCALAPPDATA -ChildPath 'PunchTrunk'
            New-Item -ItemType Directory -Path $localCacheRoot -Force | Out-Null
            $cacheLink = Join-Path -Path $localCacheRoot -ChildPath 'trunk-cache'
            if (Test-Path -Path $cacheLink) {
                if ($Force) {
                    Remove-Item -Path $cacheLink -Recurse -Force
                }
                else {
                    Write-Warning "Cache link already exists at $cacheLink. Use -Force to replace it."
                    $cacheLink = $null
                }
            }
            if ($cacheLink) {
                New-Item -ItemType Junction -Path $cacheLink -Target $cacheTarget | Out-Null
            }
        }
    }

    $envContent = @"
# PowerShell environment configuration for PunchTrunk (air-gapped)
`$env:PUNCHTRUNK_HOME = '$currentLink'
`$env:PUNCHTRUNK_TRUNK_BINARY = '$currentLink\trunk\bin\$($trunkBinary.Name)'
`$env:PUNCHTRUNK_AIRGAPPED = '1'
if (-not (`$env:PATH -split ';' | Where-Object { $_ -eq '$currentLink\bin' })) {
    `$env:PATH = '$currentLink\bin;$currentLink\trunk\bin;' + `$env:PATH
}
"@
    Set-Content -Path $EnvScript -Value $envContent -Encoding UTF8

    Write-Output "Offline PunchTrunk installed at $installFull"
    Write-Output "Wrapper scripts placed in $binDir"
    Write-Output "Environment script written to $EnvScript"
    if ($cacheCopied) {
        Write-Output "Cached trunk assets copied to $cacheTarget"
    }
}
finally {
    if (Test-Path -Path $tempRoot) {
        Remove-Item -Path $tempRoot -Recurse -Force
    }
}
