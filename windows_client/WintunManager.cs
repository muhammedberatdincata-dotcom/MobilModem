using System;
using System.Diagnostics;
using System.IO;
using System.Threading;

namespace MobilModemClient;

public class WintunManager
{
    private Process? _tunProcess;
    private bool _isRunning;

    public bool IsRunning => _isRunning;

    public bool StartTunnel(string mode, string proxyAddress, out string? error)
    {
        error = null;
        StopTunnel();

        try
        {
            string targetProxy = mode == "usb" ? "127.0.0.1:10808" : proxyAddress;
            string proxyUrl = $"socks5://{targetProxy}";

            // Locate tun2socks executable
            string? tun2SocksPath = LocateTun2Socks();
            if (string.IsNullOrEmpty(tun2SocksPath))
            {
                // If no tun2socks.exe found, create a simulated bridge or notify
                error = "tun2socks.exe bulunamadı. Lütfen dosyanın uygulama klasöründe veya %TEMP%\\MobilModem dizininde olduğundan emin olun.";
                return false;
            }

            var psi = new ProcessStartInfo
            {
                FileName = tun2SocksPath,
                Arguments = $"-device tun://MobilModem -proxy {proxyUrl} -loglevel info -mtu 1500",
                UseShellExecute = false,
                CreateNoWindow = true,
                RedirectStandardOutput = true,
                RedirectStandardError = true
            };

            _tunProcess = Process.Start(psi);
            if (_tunProcess == null || _tunProcess.HasExited)
            {
                error = "tun2socks işlemi başlatılamadı.";
                return false;
            }

            // Wait for Wintun adapter creation & assign IP
            bool ipAssigned = false;
            for (int i = 0; i < 15; i++)
            {
                Thread.Sleep(600);
                if (_tunProcess.HasExited)
                {
                    error = "tun2socks beklenmedik şekilde kapandı.";
                    return false;
                }

                if (TryAssignAdapterIp("MobilModem", "10.0.0.2", "255.255.255.0"))
                {
                    ipAssigned = true;
                    break;
                }
            }

            if (!ipAssigned)
            {
                StopTunnel();
                error = "MobilModem sanal ağ adaptörüne IP atanamadı.";
                return false;
            }

            _isRunning = true;
            return true;
        }
        catch (Exception ex)
        {
            StopTunnel();
            error = ex.Message;
            return false;
        }
    }

    public void StopTunnel()
    {
        try
        {
            if (_tunProcess != null && !_tunProcess.HasExited)
            {
                _tunProcess.Kill();
                _tunProcess.WaitForExit(2000);
            }
        }
        catch { }
        finally
        {
            _tunProcess = null;
            _isRunning = false;
        }
    }

    private static string? LocateTun2Socks()
    {
        string[] candidates = new[]
        {
            Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "tun2socks.exe"),
            Path.Combine(Path.GetTempPath(), "MobilModem", "tun2socks.exe"),
            @"c:\Users\muham\Downloads\cakmapda\tun2socks.exe"
        };

        foreach (var p in candidates)
        {
            if (File.Exists(p)) return p;
        }

        return null;
    }

    private static bool TryAssignAdapterIp(string adapterName, string ip, string mask)
    {
        try
        {
            var psi = new ProcessStartInfo
            {
                FileName = "netsh",
                Arguments = $"interface ip set address name=\"{adapterName}\" static {ip} {mask} none",
                UseShellExecute = false,
                CreateNoWindow = true,
                RedirectStandardOutput = true
            };
            using var proc = Process.Start(psi);
            proc?.WaitForExit();
            return proc?.ExitCode == 0;
        }
        catch
        {
            return false;
        }
    }
}
