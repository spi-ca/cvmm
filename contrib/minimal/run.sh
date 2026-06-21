cloud-hypervisor  --kernel vmlinuz \
                                               --cmdline 'systemd.machine_id=11111111222243338444555555555555 console=hvc0 root=/dev/vda ro init=/usr/bin/init-wrapper panic=-1' \
                                               --cpus boot=2,max=2 \
                                               --platform serial_number=11111111222243338444555555555555,uuid=11111111-2222-4333-8444-555555555555,oem_strings=amuzes-minimal \
                                               --memory size=268435456,thp=off \
                                               --disk path=root.img,readonly=on \
                                               --serial off \
                                               --console tty
