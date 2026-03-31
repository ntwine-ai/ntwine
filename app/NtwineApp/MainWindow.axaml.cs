using Avalonia.Controls;
using NtwineApp.ViewModels;

namespace NtwineApp;

public partial class MainWindow : Window
{
    public MainWindow()
    {
        InitializeComponent();
        DataContext = new MainViewModel();
    }
}
