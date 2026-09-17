using System;
using System.Diagnostics;
using System.Text.RegularExpressions;

namespace MobilModemClient;

public class TtlManager
{
    private int _originalIpv4Ttl = 128;
    private int _originalIpv6Ttl = 128;
    private bool _isModified;

    public bool IsModified => _isModified;

    public (int ipv4, int ipv6) ReadCurrentTtl()
    {
        int v4 = ReadTtl("ipv4");
        int v6 = ReadTtl("ipv6");
        return (v4, v6);
    }

    private int ReadTtl(string version)
    {
        try
        {
            var psi = new ProcessStartInfo
            {
                FileName = "netsh",
                Arguments = $"int {version} show glob",
                RedirectStandardOutput = true,
                UseShellExecute = false,
                CreateNoWindow = true
            };
            using var proc = Process.Start(psi);
            if (proc == null) return 128;
            string output = proc.StandardOutput.ReadToEnd();
            proc.WaitForExit();

            var match = Regex.Match(output, @"(?i)Default\s+(?:Cur)?Hop\s+Limit\s*:\s*(\d+)");
            if (match.Success && int.TryParse(match.Groups[1].Value, out int ttl))
            {
                return ttl;
            }
        }
        catch { }
        return 128;
    }

    public bool ApplyTtl65(out string? error)
    {
        error = null;
        try
        {
            var (v4, v6) = ReadCurrentTtl();
            _originalIpv4Ttl = v4;
            _originalIpv6Ttl = v6;

            RunNetsh($"int ipv4 set glob defaultcurhoplimit=65");
            RunNetsh($"int ipv6 set glob defaultcurhoplimit=65");

            _isModified = true;
            return true;
        }
        catch (Exception ex)
        {
            error = ex.Message;
            return false;
        }
    }

    public void RestoreOriginal()
    {
        if (!_isModified) return;

        try
        {
            RunNetsh($"int ipv4 set glob defaultcurhoplimit={_originalIpv4Ttl}");
            RunNetsh($"int ipv6 set glob defaultcurhoplimit={_originalIpv6Ttl}");
        }
        catch { }
        finally
        {
            _isModified = false;
        }
    }

    private static void RunNetsh(string args)
    {
        var psi = new ProcessStartInfo
        {
            FileName = "netsh",
            Arguments = args,
            UseShellExecute = false,
            CreateNoWindow = true
        };
        using var proc = Process.Start(psi);
        proc?.WaitForExit();
    }
}
