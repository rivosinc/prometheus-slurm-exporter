Name:       prometheus-slurm-exporter
Version:    %{version}
Release:    1
Summary:    Slurm data collector service for Prometheus
License:    Apache License 2.0
Source0:    bin/%{name}-%{version}.tar.gz

Provides:       %{name} = %{version}

ExclusiveArch:  x86_64
Requires:       systemd

%description

%global debug_package %{nil}

%prep
%autosetup

%build

%install

install -Dpm 0755 bin/%{name} %{buildroot}%{_bindir}/%{name}
install -Dpm 644 contrib/%{name}.service %{buildroot}/usr/lib/systemd/system/%{name}.service
install -Dpm 644 contrib/%{name} %{buildroot}%{_sysconfdir}/sysconfig/%{name}

%check

#-- SCRIPTLETS -----------------------------------------------------------------#
%post
systemctl enable %{name}.service
systemctl start %{name}.service

%preun
if [ $1 -eq 0 ] ; then
        # Package removal, not upgrade
        systemctl --no-reload disable --now %{name}.service &>/dev/null || :
fi

%postun
if [ $1 -ge 1 ] ; then
        # Package upgrade, not uninstall
        systemctl try-restart %{name}.service &>/dev/null || :
fi

#-- FILES ---------------------------------------------------------------------#
%files
%{_bindir}/%{name}
/usr/lib/systemd/system/%{name}.service
%{_sysconfdir}/sysconfig/%{name}

#-- CHANGELOG -----------------------------------------------------------------#
%changelog
* Thu Nov 20 2025 David Geigle <davd.geigle@eviden.com>
-initial rpm built
