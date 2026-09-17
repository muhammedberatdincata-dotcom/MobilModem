using System;
using System.Diagnostics;
using System.IO;
using System.Net;
using System.Net.Sockets;
using System.Threading;
using System.Threading.Tasks;

namespace MobilModemClient;

public class ApkManager
{
    private HttpListener? _listener;
    private CancellationTokenSource? _serverCts;
    private int _serverPort = 8085;

    public bool IsServerRunning => _listener?.IsListening == true;
    public string? DownloadUrl { get; private set; }

    public string? LocateApkFile()
    {
        // Search candidates
        string[] candidates = new[]
        {
            // Relative to app base dir
            Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "MobilModem.apk"),
            Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "app-debug.apk"),
            // In project workspace
            Path.Combine(AppDomain.CurrentDomain.BaseDirectory, "..", "..", "..", "..", "android", "app", "build", "outputs", "apk", "debug", "app-debug.apk"),
            @"c:\Users\muham\Downloads\cakmapda\android\app\build\outputs\apk\debug\app-debug.apk",
            @"c:\Users\muham\Downloads\cakmapda\MobilModem.apk"
        };

        foreach (var path in candidates)
        {
            try
            {
                string full = Path.GetFullPath(path);
                if (File.Exists(full))
                {
                    return full;
                }
            }
            catch { }
        }

        return null;
    }

    public string GetLocalIpAddress()
    {
        try
        {
            using var socket = new Socket(AddressFamily.InterNetwork, SocketType.Dgram, 0);
            socket.Connect("8.8.8.8", 65530);
            if (socket.LocalEndPoint is IPEndPoint endPoint)
            {
                return endPoint.Address.ToString();
            }
        }
        catch { }

        return "127.0.0.1";
    }

    public bool StartLocalDownloadServer(string apkPath, out string? url, out string? error)
    {
        url = null;
        error = null;

        if (!File.Exists(apkPath))
        {
            error = "APK dosyası bulunamadı.";
            return false;
        }

        StopLocalDownloadServer();

        try
        {
            _serverCts = new CancellationTokenSource();
            _listener = new HttpListener();
            string localIp = GetLocalIpAddress();
            
            _listener.Prefixes.Add($"http://*:{_serverPort}/");
            _listener.Start();

            DownloadUrl = $"http://{localIp}:{_serverPort}/MobilModem.apk";
            url = DownloadUrl;

            Task.Run(() => HandleHttpRequests(apkPath, _serverCts.Token));
            return true;
        }
        catch (Exception ex)
        {
            error = $"HTTP sunucusu başlatılamadı: {ex.Message}";
            return false;
        }
    }

    private void HandleHttpRequests(string apkPath, CancellationToken ct)
    {
        while (!ct.IsCancellationRequested && _listener != null && _listener.IsListening)
        {
            try
            {
                var context = _listener.GetContext();
                Task.Run(() =>
                {
                    try
                    {
                        var response = context.Response;
                        byte[] fileBytes = File.ReadAllBytes(apkPath);

                        response.ContentType = "application/vnd.android.package-archive";
                        response.AddHeader("Content-Disposition", "attachment; filename=\"MobilModem.apk\"");
                        response.ContentLength64 = fileBytes.Length;

                        using var output = response.OutputStream;
                        output.Write(fileBytes, 0, fileBytes.Length);
                        output.Flush();
                        response.Close();
                    }
                    catch { }
                }, ct);
            }
            catch { }
        }
    }

    public void StopLocalDownloadServer()
    {
        try
        {
            _serverCts?.Cancel();
            _listener?.Stop();
            _listener?.Close();
        }
        catch { }
        finally
        {
            _listener = null;
            _serverCts = null;
            DownloadUrl = null;
        }
    }
}
