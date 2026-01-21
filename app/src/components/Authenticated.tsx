import { type Component, type JSX, Show } from 'solid-js';
import { authState } from '../lib/auth';

interface AuthenticatedProps {
  children: JSX.Element;
}

const Authenticated: Component<AuthenticatedProps> = (props) => {

  return <Show when={authState().isAuthenticated && !authState().isLoading}>
    {props.children}
  </Show>
};

export default Authenticated;
