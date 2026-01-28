import { FiUser, FiCpu } from "solid-icons/fi";
import { authState } from "../signals/authSignals";

interface SettingsViewProps {
  nodeId: string;
}

export default function SettingsView(props: SettingsViewProps) {
  const user = () => authState().user;

  return (
    <div class="h-full flex items-center justify-center bg-neu-900">
      <div class="w-full max-w-md px-8">
        {/* User Info Section */}
        <div class="space-y-6">

          {/* User Card */}
          <div class="bg-neu-900/50 border border-neu-800 rounded-2xl p-6">
            <div class="flex items-center gap-4">
              <div class="flex-shrink-0">
                <div class="w-10 h-10 rounded-full bg-neu-800 flex items-center justify-center">
                  <FiUser class="w-5 h-5 text-neu-300" />
                </div>
              </div>
              <div class="flex-1">
                <div class="text-sm text-neu-400">Connected as</div>
                <div class="text-white/90 mt-0.5">
                  {user()?.isGuest ? "Guest" : "User"}
                </div>
              </div>
            </div>
          </div>

          {/* Node ID Card */}
          <div class="bg-neu-900/50 border border-neu-800 rounded-2xl p-6">
            <div class="flex items-start gap-4">
              <div class="flex-shrink-0 mt-1">
                <div class="w-10 h-10 rounded-full bg-neu-800 flex items-center justify-center">
                  <FiCpu class="w-5 h-5 text-neu-400" />
                </div>
              </div>
              <div class="flex-1 min-w-0">
                <div class="text-xs text-neu-500 uppercase tracking-wider mb-1">Node ID</div>
                <div class="text-sm text-white/80 font-mono truncate">
                  {props.nodeId}
                </div>
              </div>
            </div>
          </div>


        </div>
      </div>
    </div>
  );
}
