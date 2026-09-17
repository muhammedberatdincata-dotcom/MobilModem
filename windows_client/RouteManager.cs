using System;
using System.Diagnostics;
using System.IO;
using System.Linq;

namespace MobilModemClient;

public class RouteManager
{
    private string? _physicalGateway;
    private string? _proxyHost;
    private bool _routesAdded;

    public bool RoutesAdded => _routesAdded;
    public string? PhysicalGateway => _physicalGateway;

    public bool DetectPhysicalGateway(out string? error)
    {
        error = null;
        try
        {
            // First attempt: PowerShell Get-NetRoute
            var psi = new ProcessStartInfo
            {
                FileName = "powershell",
                Arguments = "-NoProfile -Command \"(Get-NetRoute -DestinationPrefix '0.0.0.0/0' | Sort-Object RouteMetric | Select-Object -First 1).NextHop\"",
                RedirectStandardOutput = true,
                UseShellExecute = false,
                CreateNoWindow = true
            };
            using var proc = Process.Start(psi);
            if (proc != null)
            {
                string gw = proc.StandardOutput.ReadToEnd().Trim();
                proc.WaitForExit();
                if (!string.IsNullOrEmpty(gw) && gw != "0.0.0.0")
                {
                    _physicalGateway = gw;
                    return true;
                }
            }

            // Fallback attempt: route print
            var psiRoute = new ProcessStartInfo
            {
                FileName = "route",
                Arguments = "print 0.0.0.0",
                RedirectStandardOutput = true,
                UseShellExecute = false,
                CreateNoWindow = true
            };
            using var procRoute = Process.Start(psiRoute);
            if (procRoute != null)
            {
                string output = procRoute.StandardOutput.ReadToEnd();
                procRoute.WaitForExit();

                using var reader = new StringReader(output);
                string? line;
                while ((line = reader.ReadLine()) != null)
                {
                    var parts = line.Trim().Split(' ', StringSplitOptions.RemoveEmptyEntries);
                    if (parts.Length >= 4 && parts[0] == "0.0.0.0" && parts[1] == "0.0.0.0")
                    {
                        _physicalGateway = parts[2];
                        return true;
                    }
                }
            }

            error = "Varsayılan ağ geçidi bulunamadı.";
            return false;
        }
        catch (Exception ex)
        {
            error = ex.Message;
            return false;
        }
    }

    public bool SetupRoutes(string proxyAddress, out string? error)
    {
        error = null;
        try
        {
            _proxyHost = proxyAddress.Split(':')[0].Trim();

            if (!DetectPhysicalGateway(out string? gwErr))
            {
                error = gwErr ?? "Ağ geçidi tespit edilemedi.";
                return false;
            }

            // 1. ROUTING LOOP PROTECTION
            // Add explicit loopback route so ADB traffic never hits Wintun
            RunRoute("add 127.0.0.0 mask 255.0.0.0 0.0.0.0 metric 1 if 1");

            // If proxy is not localhost (e.g. Wi-Fi Direct 192.168.49.1)
            if (_proxyHost != "127.0.0.1" && !string.Equals(_proxyHost, "localhost", StringComparison.OrdinalIgnoreCase))
            {
                RunRoute($"add 192.168.49.0 mask 255.255.255.0 {_proxyHost} metric 5");
                RunRoute($"add {_proxyHost} mask 255.255.255.255 {_physicalGateway} metric 5");
            }

            // 2. DNS CONFIGURATION on MobilModem Adapter
            RunCmd("netsh", "interface ip set dns name=\"MobilModem\" static 1.1.1.1");
            RunCmd("netsh", "interface ip add dns name=\"MobilModem\" 8.8.8.8 index=2");

            // 3. SPLIT ROUTE (Two-Halves Trick 0.0.0.0/1 + 128.0.0.0/1)
            RunRoute("add 0.0.0.0 mask 128.0.0.0 10.0.0.1 metric 5");
            RunRoute("add 128.0.0.0 mask 128.0.0.0 10.0.0.1 metric 5");

            _routesAdded = true;
            return true;
        }
        catch (Exception ex)
        {
            error = ex.Message;
            return false;
        }
    }

    public void Cleanup()
    {
        if (!_routesAdded) return;

        try
        {
            RunRoute("delete 0.0.0.0 mask 128.0.0.0");
            RunRoute("delete 128.0.0.0 mask 128.0.0.0");

            if (!string.IsNullOrEmpty(_proxyHost) && _proxyHost != "127.0.0.1" && !string.Equals(_proxyHost, "localhost", StringComparison.OrdinalIgnoreCase))
            {
                RunRoute($"delete {_proxyHost} mask 255.255.255.255");
                RunRoute("delete 192.168.49.0 mask 255.255.255.0");
            }

            RunRoute("delete 127.0.0.0 mask 255.0.0.0 0.0.0.0 if 1");
        }
        catch { }
        finally
        {
            _routesAdded = false;
        }
    }

    private static void RunRoute(string args) => RunCmd("route", args);

    private static void RunCmd(string exe, string args)
    {
        try
        {
            var psi = new ProcessStartInfo
            {
                FileName = exe,
                Arguments = args,
                UseShellExecute = false,
                CreateNoWindow = true
            };
            using var proc = Process.Start(psi);
            proc?.WaitForExit();
        }
        catch { }
    }
}
