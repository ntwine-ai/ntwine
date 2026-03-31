using System;
using Avalonia.Controls;
using NtwineApp.Services;
using NtwineApp.ViewModels;

namespace NtwineApp;

public partial class MainWindow : Window
{
    private readonly GoBackendProcess _backend = new("8090");

    public MainWindow()
    {
        InitializeComponent();

        var vm = new MainViewModel(_backend);
        DataContext = vm;

        Opened += async (_, _) =>
        {
            vm.StatusText = "starting backend...";
            var ok = await _backend.Start();
            vm.StatusText = ok ? "connected" : "backend offline - start manually on port 8090";
            vm.BackendConnected = ok;
        };

        Closing += (_, _) =>
        {
            _backend.Dispose();
        };
    }
}
